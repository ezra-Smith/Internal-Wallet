package logic

import (
	"context"
	"strconv"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type GetDepositAddressBalanceStatsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetDepositAddressBalanceStatsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetDepositAddressBalanceStatsLogic {
	return &GetDepositAddressBalanceStatsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetDepositAddressBalanceStatsLogic) GetDepositAddressBalanceStats(in *pb.GetDepositAddressBalanceStatsRequest) (*pb.GetDepositAddressBalanceStatsResponse, error) {
	if in == nil {
		in = &pb.GetDepositAddressBalanceStatsRequest{}
	}
	if l.svcCtx.DB == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	// 构建查询条件
	query := l.svcCtx.DB.WithContext(l.ctx).Model(&model.WalletDepositAddressBalanceModel{}).Where("deleted_at IS NULL")

	if in.UserId > 0 {
		query = query.Where("user_id = ?", in.UserId)
	}
	if strings.TrimSpace(in.AssetCode) != "" {
		query = query.Where("asset_code = ?", strings.ToUpper(strings.TrimSpace(in.AssetCode)))
	}
	if strings.TrimSpace(in.ChainCode) != "" {
		query = query.Where("chain_code = ?", strings.ToUpper(strings.TrimSpace(in.ChainCode)))
	}

	// 总地址数
	var totalAddresses int64
	if err := query.Count(&totalAddresses).Error; err != nil {
		l.Logger.Errorf("count total addresses failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	// 活跃地址数（余额 > 0）
	var activeAddresses int64
	if err := query.Where("balance_raw > 0").Count(&activeAddresses).Error; err != nil {
		l.Logger.Errorf("count active addresses failed: %v", err)
		activeAddresses = 0
	}

	// 待归集地址数
	var needsSweepCount int64
	if err := query.Where("needs_sweep = 1").Count(&needsSweepCount).Error; err != nil {
		l.Logger.Errorf("count needs sweep failed: %v", err)
		needsSweepCount = 0
	}

	// 总余额USD（DECIMAL(65,0) 使用 string 接收）
	type BalanceSum struct {
		Total string `gorm:"column:total"`
	}
	var totalBalanceSum BalanceSum
	if err := query.Select("COALESCE(SUM(balance_usd_raw), 0) as total").Scan(&totalBalanceSum).Error; err != nil {
		l.Logger.Errorf("sum total balance failed: %v", err)
		totalBalanceSum.Total = "0"
	}

	// 待归集总额（DECIMAL(65,0) 使用 string 接收）
	var pendingSweepSum BalanceSum
	if err := query.Where("needs_sweep = 1").Select("COALESCE(SUM(balance_usd_raw), 0) as total").Scan(&pendingSweepSum).Error; err != nil {
		l.Logger.Errorf("sum pending sweep failed: %v", err)
		pendingSweepSum.Total = "0"
	}

	// Sweep Tasks 已移除：这些字段固定返回 0
	last24hSweepCount := int64(0)
	sweepSuccessRate := float64(0)

	// 将 string 转换为 int64 用于 centsToUSDString（如果值超过 int64 范围，会溢出）
	totalBalanceUSDRawInt64 := int64(0)
	if totalBalanceSum.Total != "" {
		if val, err := strconv.ParseInt(totalBalanceSum.Total, 10, 64); err == nil {
			totalBalanceUSDRawInt64 = val
		}
	}
	pendingSweepUSDRawInt64 := int64(0)
	if pendingSweepSum.Total != "" {
		if val, err := strconv.ParseInt(pendingSweepSum.Total, 10, 64); err == nil {
			pendingSweepUSDRawInt64 = val
		}
	}

	return &pb.GetDepositAddressBalanceStatsResponse{
		Success: true,
		Message: "ok",
		Data: &pb.GetDepositAddressBalanceStatsData{
			TotalAddresses:     totalAddresses,
			ActiveAddresses:    activeAddresses,
			NeedsSweepCount:    needsSweepCount,
			TotalBalanceUsd:    centsToUSDString(totalBalanceUSDRawInt64),
			TotalBalanceUsdRaw: totalBalanceSum.Total,
			PendingSweepUsd:    centsToUSDString(pendingSweepUSDRawInt64),
			PendingSweepUsdRaw: pendingSweepSum.Total,
			Last_24HSweepCount: last24hSweepCount,
			SweepSuccessRate:   sweepSuccessRate,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
