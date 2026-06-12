package logic

import (
	"context"
	admininterceptor "internalwallet/services/admin/rpc/internal/interceptor"
	"strings"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type UpdateDepositAddressBalanceLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateDepositAddressBalanceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateDepositAddressBalanceLogic {
	return &UpdateDepositAddressBalanceLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateDepositAddressBalanceLogic) UpdateDepositAddressBalance(in *pb.UpdateDepositAddressBalanceRequest) (*pb.UpdateDepositAddressBalanceResponse, error) {
	if in == nil || in.DepositAddressId <= 0 || strings.TrimSpace(in.AssetCode) == "" || strings.TrimSpace(in.ChainCode) == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "deposit_address_id/asset_code/chain_code required", nil)
	}
	if l.svcCtx.DB == nil || l.svcCtx.WalletDepositAddressBalanceRepo == nil || l.svcCtx.WalletDepositAddressRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	// Ensure authenticated (RBAC already enforced by interceptor)
	if _, ok := admininterceptor.GetCurrentAdmin(l.ctx); !ok {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	// 构建余额模型
	now := time.Now()
	syncStatus := strings.TrimSpace(in.SyncStatus)
	if syncStatus == "" {
		syncStatus = "synced"
	}

	var syncError *string
	if strings.TrimSpace(in.SyncError) != "" {
		err := strings.TrimSpace(in.SyncError)
		syncError = &err
	}

	// proto 字段现在是 string，直接使用（数据库也是 DECIMAL(65,0)）
	balance := &model.WalletDepositAddressBalanceModel{
		DepositAddressID:  in.DepositAddressId,
		UserID:            in.UserId,
		AssetCode:         strings.ToUpper(strings.TrimSpace(in.AssetCode)),
		ChainCode:         strings.ToUpper(strings.TrimSpace(in.ChainCode)),
		Balance:           in.Balance,
		BalanceRaw:        in.BalanceRaw,
		BalanceUSD:        in.BalanceUsd,
		BalanceUSDRaw:     in.BalanceUsdRaw,
		SweepThresholdRaw: in.SweepThresholdRaw,
		LastSyncedAt:      &now,
		SyncStatus:        syncStatus,
		SyncError:         syncError,
	}

	// Upsert 余额（自动判断是否需要归集）
	if err := l.svcCtx.WalletDepositAddressBalanceRepo.Upsert(l.ctx, balance); err != nil {
		l.Logger.Errorf("upsert deposit address balance failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "update balance failed", nil)
	}

	// 查询更新后的记录
	updated, err := l.svcCtx.WalletDepositAddressBalanceRepo.FindByDepositAddressAssetChain(l.ctx, in.DepositAddressId, balance.AssetCode, balance.ChainCode)
	if err != nil {
		l.Logger.Errorf("find updated balance failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	// 查询充值地址信息
	address := ""
	addr, err := l.svcCtx.WalletDepositAddressRepo.FindByID(l.ctx, in.DepositAddressId)
	if err == nil && addr != nil {
		address = addr.Address
	}

	markedForSweep := updated.NeedsSweep == 1

	return &pb.UpdateDepositAddressBalanceResponse{
		Success: true,
		Message: "ok",
		Data: &pb.UpdateDepositAddressBalanceData{
			Balance:        toPBDepositAddressBalanceItem(updated, address),
			MarkedForSweep: markedForSweep,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
