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

type GetVaultDetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetVaultDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetVaultDetailLogic {
	return &GetVaultDetailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetVaultDetailLogic) GetVaultDetail(in *pb.GetVaultDetailRequest) (*pb.GetVaultDetailResponse, error) {
	if in == nil {
		in = &pb.GetVaultDetailRequest{}
	}
	if l.svcCtx.DB == nil ||
		l.svcCtx.AccountingRpc == nil ||
		l.svcCtx.VaultNetworkRepo == nil ||
		l.svcCtx.VaultBalanceRepo == nil ||
		l.svcCtx.VaultThresholdRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	chainID := in.ChainId
	network, err := l.svcCtx.VaultNetworkRepo.FindByChainID(l.ctx, chainID)
	if err != nil || network == nil {
		return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "NETWORK_NOT_FOUND", "network not found", map[string]string{"chain_id": "not found"})
	}

	balances, err := l.svcCtx.VaultBalanceRepo.ListByNetworkID(l.ctx, network.ID)
	if err != nil {
		l.Logger.Errorf("list vault balances failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	thresholds, err := l.svcCtx.VaultThresholdRepo.ListByNetworkID(l.ctx, network.ID)
	if err != nil {
		l.Logger.Errorf("list vault thresholds failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}
	thByCurrency := make(map[string]*model.VaultThresholdModel, len(thresholds))
	for _, th := range thresholds {
		if th == nil {
			continue
		}
		thByCurrency[normalizeCode(th.Currency)] = th
	}

	// Aggregate pending decrease adjustments as "locked" funds.
	type pendingAgg struct {
		Currency string `gorm:"column:currency"`
		Cnt      int64  `gorm:"column:cnt"`
		SumRaw   int64  `gorm:"column:sum_raw"`
	}
	var aggs []pendingAgg
	if err := l.svcCtx.DB.WithContext(l.ctx).Model(&model.VaultAdjustmentModel{}).
		Select("currency as currency, COUNT(1) as cnt, COALESCE(SUM(amount_raw),0) as sum_raw").
		Where(
			"network_id = ? AND adjustment_type = ? AND status IN ? AND deleted_at IS NULL",
			network.ID,
			vaultAdjustmentTypeDecrease,
			[]string{vaultAdjustmentStatusPending, vaultAdjustmentStatusApproved, vaultAdjustmentStatusProcessing},
		).
		Group("currency").
		Scan(&aggs).Error; err != nil {
		l.Logger.Errorf("aggregate pending adjustments failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	pendingCountByCurrency := make(map[string]int64, len(aggs))
	pendingSumRawByCurrency := make(map[string]int64, len(aggs))
	for _, a := range aggs {
		ccy := normalizeCode(a.Currency)
		pendingCountByCurrency[ccy] = a.Cnt
		pendingSumRawByCurrency[ccy] = a.SumRaw
	}

	// Load asset precision for formatting.
	currencyCodes := make([]string, 0, len(balances))
	seenCcy := map[string]struct{}{}
	for _, b := range balances {
		if b == nil {
			continue
		}
		ccy := normalizeCode(b.Currency)
		if ccy == "" {
			continue
		}
		if _, ok := seenCcy[ccy]; ok {
			continue
		}
		seenCcy[ccy] = struct{}{}
		currencyCodes = append(currencyCodes, ccy)
	}
	precisionByCurrency := map[string]int32{}
	if len(currencyCodes) > 0 {
		for _, code := range currencyCodes {
			code = normalizeCode(code)
			if code == "" {
				continue
			}
			accResp, err := l.svcCtx.AccountingRpc.GetAsset(l.ctx, &pb.GetAssetRequest{Code: code})
			if err != nil {
				l.Logger.Errorf("call accounting GetAsset failed: %v", err)
				return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "accounting service error", nil)
			}
			if accResp == nil || !accResp.Success || accResp.Item == nil {
				continue
			}
			precisionByCurrency[normalizeCode(accResp.Item.Code)] = accResp.Item.Precision
		}
	}

	respBalances := make([]*pb.VaultBalanceDetailItem, 0, len(balances))
	worst := vaultCurrencyStatusNormal
	for _, b := range balances {
		if b == nil {
			continue
		}
		ccy := normalizeCode(b.Currency)
		precision, ok := precisionByCurrency[ccy]
		if !ok {
			precision = 6
		}
		contract := strings.TrimSpace(b.ContractAddress)

		lockedRaw := pendingSumRawByCurrency[ccy]
		availableRaw := b.BalanceRaw - lockedRaw
		if availableRaw < 0 {
			availableRaw = 0
		}

		var low, critical string
		var lowRaw, criticalRaw int64
		var notifyEmail, notifyWebhook bool
		if th := thByCurrency[ccy]; th != nil {
			low = th.ThresholdLow
			critical = th.ThresholdCritical
			lowRaw = th.ThresholdLowRaw
			criticalRaw = th.ThresholdCriticalRaw

			notifyCfg := parseVaultNotificationsJSON(th.Notifications)
			notifyEmail = notifyCfg.EmailEnabled
			notifyWebhook = notifyCfg.WebhookEnabled
		}

		curStatus := vaultAmountStatus(b.BalanceRaw, lowRaw, criticalRaw)
		if curStatus == vaultCurrencyStatusCritical {
			worst = vaultCurrencyStatusCritical
		} else if curStatus == vaultCurrencyStatusLow && worst == vaultCurrencyStatusNormal {
			worst = vaultCurrencyStatusLow
		}

		var thPB *pb.VaultThresholdDetail
		if low != "" || critical != "" || notifyEmail || notifyWebhook {
			thPB = &pb.VaultThresholdDetail{
				Low:           low,
				Critical:      critical,
				NotifyEmail:   notifyEmail,
				NotifyWebhook: notifyWebhook,
			}
		}

		respBalances = append(respBalances, &pb.VaultBalanceDetailItem{
			Currency:                 b.Currency,
			ContractAddress:          contract,
			Balance:                  b.Balance,
			BalanceUsd:               b.BalanceUSD,
			LockedBalance:            rawToFixedAmountString(lockedRaw, precision),
			AvailableBalance:         rawToFixedAmountString(availableRaw, precision),
			PendingWithdrawals:       int32(pendingCountByCurrency[ccy]),
			PendingWithdrawalsAmount: rawToFixedAmountString(lockedRaw, precision),
			ThresholdConfig:          thPB,
			Status:                   curStatus,
		})
	}

	syncStatus := strings.TrimSpace(network.SyncStatus)
	if syncStatus == "" {
		syncStatus = "unknown"
	}

	netStatus := vaultNetworkStatusHealthy
	switch worst {
	case vaultCurrencyStatusCritical:
		netStatus = vaultNetworkStatusCritical
	case vaultCurrencyStatusLow:
		netStatus = vaultNetworkStatusWarning
	}
	switch syncStatus {
	case "delayed", "error":
		if netStatus == vaultNetworkStatusHealthy {
			netStatus = vaultNetworkStatusWarning
		}
	}

	// Recent transactions: reuse adjustment history as best-effort.
	var recent []*model.VaultAdjustmentModel
	if err := l.svcCtx.DB.WithContext(l.ctx).
		Where("network_id = ? AND deleted_at IS NULL", network.ID).
		Order("created_at DESC").
		Limit(10).
		Find(&recent).Error; err != nil {
		l.Logger.Errorf("load recent adjustments failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	recentPB := make([]*pb.VaultRecentTransactionItem, 0, len(recent))
	for _, r := range recent {
		if r == nil {
			continue
		}
		typ := "outflow"
		if r.AdjustmentType == vaultAdjustmentTypeIncrease {
			typ = "inflow"
		}
		toAddr := ""
		if r.AdjustmentType == vaultAdjustmentTypeDecrease {
			toAddr = derefString(r.SourceAddress)
		}
		recentPB = append(recentPB, &pb.VaultRecentTransactionItem{
			Id:        fmt.Sprintf("adj-%d", r.ID),
			Type:      typ,
			Currency:  r.Currency,
			Amount:    r.Amount,
			ToAddress: toAddr,
			TxHash:    derefString(r.TxHash),
			Status:    r.Status,
			CreatedAt: formatTimePtr(r.CreatedAt),
		})
	}

	var blocksBehind int64
	if network.CurrentBlock != nil && network.LastBlockSynced != nil && *network.CurrentBlock > *network.LastBlockSynced {
		blocksBehind = *network.CurrentBlock - *network.LastBlockSynced
	}
	syncInfo := &pb.VaultSyncInfo{
		LastSyncAt:      formatTimePtr(network.LastSyncAt),
		LastBlockSynced: derefInt64(network.LastBlockSynced),
		CurrentBlock:    derefInt64(network.CurrentBlock),
		BlocksBehind:    blocksBehind,
		SyncStatus:      syncStatus,
	}

	// Gas info: best-effort (native token = contract_address empty).
	gas := &pb.VaultGasInfo{
		Balance:               "0",
		BalanceUsd:            "0",
		EstimatedTransactions: 0,
		AvgGasPriceGwei:       "",
	}
	for _, b := range balances {
		if b == nil {
			continue
		}
		if strings.TrimSpace(b.ContractAddress) == "" {
			gas.Balance = b.Balance
			gas.BalanceUsd = b.BalanceUSD
			break
		}
	}

	return &pb.GetVaultDetailResponse{
		Success: true,
		Message: "ok",
		Data: &pb.GetVaultDetailData{
			Network:            network.Network,
			ChainId:            network.ChainID,
			VaultAddress:       derefString(network.VaultAddress),
			Status:             netStatus,
			Balances:           respBalances,
			RecentTransactions: recentPB,
			SyncInfo:           syncInfo,
			GasInfo:            gas,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}

func derefInt64(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}
