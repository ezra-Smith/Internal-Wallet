package logic

import (
	"context"
	"fmt"
	"strconv"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type GetDepositAddressBalanceByAssetLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetDepositAddressBalanceByAssetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetDepositAddressBalanceByAssetLogic {
	return &GetDepositAddressBalanceByAssetLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetDepositAddressBalanceByAsset 按币种统计待归集金额（币种分布）
func (l *GetDepositAddressBalanceByAssetLogic) GetDepositAddressBalanceByAsset(in *pb.GetDepositAddressBalanceByAssetRequest) (*pb.GetDepositAddressBalanceByAssetResponse, error) {
	if in == nil {
		in = &pb.GetDepositAddressBalanceByAssetRequest{}
	}
	if l.svcCtx.DB == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	// 按币种聚合统计
	// 注意：balance_raw 和 balance_usd_raw 是 DECIMAL(65,0)，使用 string 避免溢出
	type AssetAggregation struct {
		AssetCode          string `gorm:"column:asset_code"`
		AddressCount       int64  `gorm:"column:address_count"`
		TotalBalanceRaw    string `gorm:"column:total_balance_raw;type:decimal(65,0)"`
		TotalBalanceUSDRaw string `gorm:"column:total_balance_usd_raw;type:decimal(65,0)"`
	}

	var aggregations []AssetAggregation
	query := l.svcCtx.DB.WithContext(l.ctx).
		Model(&model.WalletDepositAddressBalanceModel{}).
		Select(`
			asset_code,
			COUNT(DISTINCT id) as address_count,
			COALESCE(SUM(balance_raw), 0) as total_balance_raw,
			COALESCE(SUM(balance_usd_raw), 0) as total_balance_usd_raw
		`).
		Where("deleted_at IS NULL").
		Where("balance_raw > 0"). // 只统计有余额的地址
		Group("asset_code").
		Order("total_balance_usd_raw DESC") // 按USD金额降序

	// 可选筛选特定币种
	if in.AssetCode != "" {
		query = query.Where("asset_code = ?", in.AssetCode)
	}

	if err := query.Scan(&aggregations).Error; err != nil {
		l.Logger.Errorf("query asset aggregations failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	// 计算总USD金额（使用 decimal 进行累加）
	totalBalanceUSDRaw := decimal.Zero
	for _, agg := range aggregations {
		if balanceUSDRaw, err := decimal.NewFromString(agg.TotalBalanceUSDRaw); err == nil {
			totalBalanceUSDRaw = totalBalanceUSDRaw.Add(balanceUSDRaw)
		}
	}

	// 转换为 protobuf 格式
	assets := make([]*pb.AssetBalanceStats, 0, len(aggregations))
	for _, agg := range aggregations {
		// 计算百分比（使用 decimal 进行精确计算）
		var percentage string
		if !totalBalanceUSDRaw.IsZero() {
			aggUSDRaw, _ := decimal.NewFromString(agg.TotalBalanceUSDRaw)
			if !aggUSDRaw.IsZero() {
				pct := aggUSDRaw.Div(totalBalanceUSDRaw).Mul(decimal.NewFromInt(100))
				percentage = fmt.Sprintf("%.2f%%", pct.InexactFloat64())
			} else {
				percentage = "0.00%"
			}
		} else {
			percentage = "0.00%"
		}

		// 格式化余额（TotalBalanceRaw 现在是 string）
		totalBalance := formatRawBalanceFromString(agg.TotalBalanceRaw, agg.AssetCode)

		// 将 TotalBalanceUSDRaw 转换为 int64 用于 centsToUSDString（如果值超过 int64 范围，会溢出）
		aggUSDRawInt64 := int64(0)
		if val, err := strconv.ParseInt(agg.TotalBalanceUSDRaw, 10, 64); err == nil {
			aggUSDRawInt64 = val
		}

		assets = append(assets, &pb.AssetBalanceStats{
			AssetCode:          agg.AssetCode,
			AssetName:          agg.AssetCode, // 可以从币种配置表获取名称
			AddressCount:       agg.AddressCount,
			TotalBalance:       totalBalance,
			TotalBalanceRaw:    agg.TotalBalanceRaw,
			TotalBalanceUsd:    centsToUSDString(aggUSDRawInt64),
			TotalBalanceUsdRaw: agg.TotalBalanceUSDRaw,
			Percentage:         percentage,
		})
	}

	// 将 totalBalanceUSDRaw 转换为 int64 用于 centsToUSDString（如果值超过 int64 范围，会溢出）
	totalBalanceUSDRawInt64 := int64(0)
	if !totalBalanceUSDRaw.IsZero() {
		if val, err := strconv.ParseInt(totalBalanceUSDRaw.String(), 10, 64); err == nil {
			totalBalanceUSDRawInt64 = val
		}
	}

	return &pb.GetDepositAddressBalanceByAssetResponse{
		Success: true,
		Message: "ok",
		Data: &pb.GetDepositAddressBalanceByAssetData{
			Assets:             assets,
			TotalBalanceUsd:    centsToUSDString(totalBalanceUSDRawInt64),
			TotalBalanceUsdRaw: totalBalanceUSDRaw.String(),
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}

// formatRawBalanceFromString 格式化余额（从 string 格式的 raw balance）
func formatRawBalanceFromString(rawBalanceStr string, assetCode string) string {
	// 大部分币种都是18位精度，简化处理
	// 实际应该从配置表读取精度
	rawBalance, err := decimal.NewFromString(rawBalanceStr)
	if err != nil {
		return "0"
	}
	// 除以 10^18 转换为可读格式
	balance := rawBalance.Div(decimal.NewFromInt(1e18))
	return balance.StringFixed(8)
}

// formatRawBalance 格式化余额（简化版，实际可能需要根据币种精度处理）
// 保留此函数以保持向后兼容
func formatRawBalance(rawBalance int64, assetCode string) string {
	// 大部分币种都是18位精度，简化处理
	// 实际应该从配置表读取精度
	balance := float64(rawBalance) / 1e18
	return fmt.Sprintf("%.8f", balance)
}
