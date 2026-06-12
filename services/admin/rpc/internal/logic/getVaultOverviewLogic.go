package logic

import (
	"context"
	"fmt"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type GetVaultOverviewLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetVaultOverviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetVaultOverviewLogic {
	return &GetVaultOverviewLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetVaultOverviewLogic) GetVaultOverview(in *pb.GetVaultOverviewRequest) (*pb.GetVaultOverviewResponse, error) {
	if l.svcCtx.DB == nil ||
		l.svcCtx.VaultNetworkRepo == nil ||
		l.svcCtx.VaultBalanceRepo == nil ||
		l.svcCtx.VaultThresholdRepo == nil ||
		l.svcCtx.VaultAdjustmentRepo == nil ||
		l.svcCtx.WalletDepositAddressBalanceRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	networks, err := l.svcCtx.VaultNetworkRepo.List(l.ctx, "active")
	if err != nil {
		l.Logger.Errorf("list vault networks failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	pendingCount, err := l.svcCtx.VaultAdjustmentRepo.CountByStatuses(l.ctx, []string{vaultAdjustmentStatusPending})
	if err != nil {
		l.Logger.Errorf("count pending adjustments failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	totalUSDcents := int64(0)
	healthyNetworks := int32(0)
	alertsCount := int32(0)

	respNetworks := make([]*pb.VaultNetworkOverviewItem, 0, len(networks))
	for _, n := range networks {
		if n == nil {
			continue
		}

		balances, bErr := l.svcCtx.VaultBalanceRepo.ListByNetworkID(l.ctx, n.ID)
		if bErr != nil {
			l.Logger.Errorf("list vault balances failed: %v", bErr)
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
		}

		thresholds, tErr := l.svcCtx.VaultThresholdRepo.ListByNetworkID(l.ctx, n.ID)
		if tErr != nil {
			l.Logger.Errorf("list vault thresholds failed: %v", tErr)
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
		}

		thByCurrency := make(map[string]*model.VaultThresholdModel, len(thresholds))
		for _, th := range thresholds {
			if th == nil {
				continue
			}
			thByCurrency[strings.TrimSpace(th.Currency)] = th
		}

		netUSDcents := int64(0)
		balanceItems := make([]*pb.VaultBalanceOverviewItem, 0, len(balances))

		worst := vaultCurrencyStatusNormal
		for _, b := range balances {
			if b == nil {
				continue
			}
			netUSDcents += b.BalanceUSDRaw

			var low, critical string
			var lowRaw, criticalRaw int64
			if th := thByCurrency[strings.TrimSpace(b.Currency)]; th != nil {
				low = th.ThresholdLow
				critical = th.ThresholdCritical
				lowRaw = th.ThresholdLowRaw
				criticalRaw = th.ThresholdCriticalRaw
			}

			curStatus := vaultAmountStatus(b.BalanceRaw, lowRaw, criticalRaw)
			if curStatus == vaultCurrencyStatusCritical {
				worst = vaultCurrencyStatusCritical
			} else if curStatus == vaultCurrencyStatusLow && worst == vaultCurrencyStatusNormal {
				worst = vaultCurrencyStatusLow
			}

			balanceItems = append(balanceItems, &pb.VaultBalanceOverviewItem{
				Currency:          b.Currency,
				Balance:           b.Balance,
				BalanceUsd:        b.BalanceUSD,
				ThresholdLow:      low,
				ThresholdCritical: critical,
				Status:            curStatus,
			})
		}

		totalUSDcents += netUSDcents

		syncStatus := strings.TrimSpace(n.SyncStatus)
		if syncStatus == "" {
			syncStatus = "unknown"
		}

		networkStatus := vaultNetworkStatusHealthy
		var alert *pb.VaultAlert
		switch worst {
		case vaultCurrencyStatusCritical:
			networkStatus = vaultNetworkStatusCritical
		case vaultCurrencyStatusLow:
			networkStatus = vaultNetworkStatusWarning
		}
		switch syncStatus {
		case "delayed":
			if networkStatus == vaultNetworkStatusHealthy {
				networkStatus = vaultNetworkStatusWarning
			}
			alert = &pb.VaultAlert{Type: "sync_delay", Message: resp.Msg(l.ctx, "SYNC_DELAY")}
		case "error":
			if networkStatus == vaultNetworkStatusHealthy {
				networkStatus = vaultNetworkStatusWarning
			}
			alert = &pb.VaultAlert{Type: "sync_error", Message: resp.Msg(l.ctx, "SYNC_ERROR")}
		}

		if networkStatus == vaultNetworkStatusHealthy {
			healthyNetworks++
		} else {
			alertsCount++
		}

		respNetworks = append(respNetworks, &pb.VaultNetworkOverviewItem{
			Network:         n.Network,
			ChainId:         n.ChainID,
			VaultAddress:    derefString(n.VaultAddress),
			Status:          networkStatus,
			TotalBalanceUsd: centsToUSDString(netUSDcents),
			Balances:        balanceItems,
			LastSyncAt:      formatTimePtr(n.LastSyncAt),
			SyncStatus:      syncStatus,
			Alert:           alert,
			NetworkId:       n.ID,
		})
	}

	// 计算未归集钱包总资产（从 wallet_deposit_address_balance 表查询待归集余额）
	unsweptBalanceUSDRaw := l.getUnsweptBalanceTotal()

	summary := &pb.VaultOverviewSummary{
		TotalBalanceUsd:    centsToUSDString(totalUSDcents),
		TotalNetworks:      int32(len(respNetworks)),
		HealthyNetworks:    healthyNetworks,
		PendingAdjustments: int32(pendingCount),
		AlertsCount:        alertsCount,
		UnsweptBalanceUsd:  centsToUSDString(unsweptBalanceUSDRaw),
	}

	return &pb.GetVaultOverviewResponse{
		Success: true,
		Message: fmt.Sprintf("ok (%d)", len(respNetworks)),
		Data: &pb.GetVaultOverviewData{
			Summary:  summary,
			Networks: respNetworks,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}

// getUnsweptBalanceTotal 计算所有待归集地址的余额总和（USD cents）
func (l *GetVaultOverviewLogic) getUnsweptBalanceTotal() int64 {
	// 查询所有 needs_sweep = 1 的地址余额
	query := l.svcCtx.DB.WithContext(l.ctx).
		Model(&model.WalletDepositAddressBalanceModel{}).
		Where("needs_sweep = 1").
		Where("deleted_at IS NULL")

	var totalUSDRaw int64
	if err := query.Select("COALESCE(SUM(balance_usd_raw), 0)").Scan(&totalUSDRaw).Error; err != nil {
		l.Logger.Errorf("calculate unswept balance failed: %v", err)
		return 0
	}

	return totalUSDRaw
}
