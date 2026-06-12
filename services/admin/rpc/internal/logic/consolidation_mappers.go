package logic

import (
	"context"
	"strings"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/shopspring/decimal"
)

func unixSecondsToTimePtr(ts int64) *time.Time {
	if ts <= 0 {
		return nil
	}
	t := time.Unix(ts, 0).UTC()
	return &t
}

func guessAssetPrecision(chain string, assetSymbol string) int32 {
	chain = strings.ToUpper(strings.TrimSpace(chain))
	assetSymbol = strings.ToUpper(strings.TrimSpace(assetSymbol))

	// Common stablecoins.
	switch assetSymbol {
	case "USDT", "USDC":
		return 6
	}

	switch chain {
	case "TRON":
		// TRX & most TRC20 tokens are 6.
		return 6
	case "BTC":
		return 8
	case "ETH", "BSC", "POLYGON", "ARBITRUM", "OPTIMISM":
		return 18
	default:
		// Safe-ish default for EVM-like assets.
		return 18
	}
}

func formatSmallestUnitDecimal(raw string, precision int32) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	d, err := decimal.NewFromString(raw)
	if err != nil {
		// Fallback: keep original.
		return raw
	}
	if precision <= 0 {
		return d.String()
	}
	return d.Shift(-precision).String()
}

func newAssetPrecisionResolver(ctx context.Context, svcCtx *svc.ServiceContext) func(chain, assetSymbol string) int32 {
	cache := map[string]int32{}
	return func(chain, assetSymbol string) int32 {
		assetSymbol = strings.ToUpper(strings.TrimSpace(assetSymbol))
		if assetSymbol == "" {
			return guessAssetPrecision(chain, assetSymbol)
		}
		if v, ok := cache[assetSymbol]; ok {
			return v
		}
		// Default to guess; overwrite if we can fetch from Accounting.
		precision := guessAssetPrecision(chain, assetSymbol)
		if svcCtx != nil && svcCtx.AccountingRpc != nil {
			if resp, err := svcCtx.AccountingRpc.GetAsset(ctx, &pb.GetAssetRequest{Code: assetSymbol}); err == nil && resp != nil && resp.Success && resp.Item != nil {
				if resp.Item.Precision >= 0 && resp.Item.Precision <= 30 {
					precision = resp.Item.Precision
				}
			}
		}
		cache[assetSymbol] = precision
		return precision
	}
}

func toAdminConsolidationTaskItem(t *pb.ConsolidationTask, assetPrecision int32) *pb.AdminConsolidationTaskItem {
	if t == nil {
		return nil
	}
	return &pb.AdminConsolidationTaskItem{
		TaskId:               t.TaskId,
		Chain:                t.Chain,
		AssetSymbol:          t.AssetSymbol,
		TokenContract:        t.TokenContract,
		FromAddress:          t.FromAddress,
		ToAddress:            t.ToAddress,
		Amount:               t.Amount,
		EstimatedFee:         t.EstimatedFee,
		ActualFee:            t.ActualFee,
		Status:               int32(t.Status),
		TxHash:               t.TxHash,
		RetryCount:           t.RetryCount,
		ErrorMessage:         t.ErrorMessage,
		StartedAt:            formatTimePtr(unixSecondsToTimePtr(t.StartedAt)),
		ConfirmedAt:          formatTimePtr(unixSecondsToTimePtr(t.ConfirmedAt)),
		NextAttemptAt:        formatTimePtr(unixSecondsToTimePtr(t.NextAttemptAt)),
		ClaimedBy:            t.ClaimedBy,
		ClaimedUntil:         formatTimePtr(unixSecondsToTimePtr(t.ClaimedUntil)),
		EnergyRentalId:       t.EnergyRentalId,
		EnergyOrderCount:     t.EnergyOrderCount,
		EnergyTotalCostSun:   t.EnergyTotalCostSun,
		CreatedAt:            formatTimePtr(unixSecondsToTimePtr(t.CreatedAt)),
		UpdatedAt:            formatTimePtr(unixSecondsToTimePtr(t.UpdatedAt)),
		AmountReadable:       formatSmallestUnitDecimal(t.Amount, assetPrecision),
		EstimatedFeeReadable: formatSmallestUnitDecimal(t.EstimatedFee, assetPrecision),
		ActualFeeReadable:    formatSmallestUnitDecimal(t.ActualFee, assetPrecision),
	}
}

func toAdminConsolidationLogItem(l *pb.ConsolidationLog) *pb.AdminConsolidationLogItem {
	if l == nil {
		return nil
	}
	return &pb.AdminConsolidationLogItem{
		Id:        l.Id,
		TaskId:    l.TaskId,
		LogLevel:  l.LogLevel,
		Message:   l.Message,
		Details:   l.Details,
		CreatedAt: formatTimePtr(unixSecondsToTimePtr(l.CreatedAt)),
	}
}

func toAdminConsolidationTopupRecordItem(r *pb.ConsolidationTopupRecord, assetPrecision int32) *pb.AdminConsolidationTopupRecordItem {
	if r == nil {
		return nil
	}
	return &pb.AdminConsolidationTopupRecordItem{
		Id:             r.Id,
		TaskId:         r.TaskId,
		Chain:          r.Chain,
		AssetSymbol:    r.AssetSymbol,
		FromAddress:    r.FromAddress,
		ToAddress:      r.ToAddress,
		Amount:         r.Amount,
		Purpose:        r.Purpose,
		Status:         int32(r.Status),
		TxHash:         r.TxHash,
		RetryCount:     r.RetryCount,
		ErrorMessage:   r.ErrorMessage,
		ConfirmedAt:    formatTimePtr(unixSecondsToTimePtr(r.ConfirmedAt)),
		CreatedAt:      formatTimePtr(unixSecondsToTimePtr(r.CreatedAt)),
		UpdatedAt:      formatTimePtr(unixSecondsToTimePtr(r.UpdatedAt)),
		AmountReadable: formatSmallestUnitDecimal(r.Amount, assetPrecision),
	}
}

func toAdminEnergyRentalRecordItem(r *pb.EnergyRentalRecord) *pb.AdminEnergyRentalRecordItem {
	if r == nil {
		return nil
	}
	return &pb.AdminEnergyRentalRecordItem{
		Id:               r.Id,
		OrderId:          r.OrderId,
		ReceiverAddress:  r.ReceiverAddress,
		EnergyAmount:     r.EnergyAmount,
		RentalDuration:   r.RentalDuration,
		PricePerEnergy:   r.PricePerEnergy,
		TotalCost:        r.TotalCost,
		Provider:         r.Provider,
		Status:           int32(r.Status),
		ProviderResponse: r.ProviderResponse,
		ErrorMessage:     r.ErrorMessage,
		ConfirmedAt:      formatTimePtr(unixSecondsToTimePtr(r.ConfirmedAt)),
		CreatedAt:        formatTimePtr(unixSecondsToTimePtr(r.CreatedAt)),
		UpdatedAt:        formatTimePtr(unixSecondsToTimePtr(r.UpdatedAt)),
	}
}
