package logic

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/svc"
)

type GetExchangeRateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetExchangeRateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetExchangeRateLogic {
	return &GetExchangeRateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetExchangeRate 获取两个币种之间的汇率
// 支持加密货币之间的汇率（如 ETH/USDT, BTC/ETH）和加密货币与法币之间的汇率（如 ETH/CNY, BTC/USD）
// 从 market 服务存储的 Redis 数据中获取实时价格并计算汇率
// - 加密货币价格从 binance:tickers 获取（如 ETHUSDT）
// - 法币汇率从 fiat:tickers 获取（如 USDTCNY），然后转换为法币/USDT 价格
func (l *GetExchangeRateLogic) GetExchangeRate(in *pb.GetExchangeRateReq) (*pb.GetExchangeRateResp, error) {
	if in == nil {
		return nil, errx.InvalidParam("invalid params")
	}

	fromAsset := strings.ToUpper(strings.TrimSpace(in.FromAsset))
	toAsset := strings.ToUpper(strings.TrimSpace(in.ToAsset))

	if fromAsset == "" || toAsset == "" {
		return nil, errx.InvalidParam("from_asset and to_asset are required")
	}

	if fromAsset == toAsset {
		return &pb.GetExchangeRateResp{
			Success:      true,
			Message:      "success",
			FromAsset:    fromAsset,
			ToAsset:      toAsset,
			ExchangeRate: "1",
			Timestamp:    time.Now().UnixMilli(),
		}, nil
	}

	// 从 Redis 获取两个币种对 USDT 的价格
	var fromPrice, toPrice decimal.Decimal
	var fromOk, toOk bool
	var timestamp int64

	if l.svcCtx.RedisClient == nil {
		return &pb.GetExchangeRateResp{
			Success: false,
			Message: "price service unavailable",
		}, nil
	}

	// 获取源币种价格
	// 对于 USD，使用 binance:tickers 的真实市场价格（USDTUSD 的倒数）
	fromPrice, fromOk = l.getAssetPriceUSDT(fromAsset)
	if !fromOk {
		return &pb.GetExchangeRateResp{
			Success: false,
			Message: "price not found for " + fromAsset,
		}, nil
	}

	// 获取目标币种价格
	toPrice, toOk = l.getAssetPriceUSDT(toAsset)
	if !toOk {
		return &pb.GetExchangeRateResp{
			Success: false,
			Message: "price not found for " + toAsset,
		}, nil
	}

	// 计算汇率：1 from_asset = (from_price_usdt / to_price_usdt) to_asset
	// 例如：1 BTC = (BTC_USDT / ETH_USDT) ETH
	if toPrice.IsZero() {
		return &pb.GetExchangeRateResp{
			Success: false,
			Message: "invalid price for " + toAsset,
		}, nil
	}

	exchangeRate := fromPrice.Div(toPrice)

	// 获取时间戳（从 Redis 中获取最新价格的时间戳）
	// 这里简化处理，使用当前时间
	timestamp = time.Now().UnixMilli()

	return &pb.GetExchangeRateResp{
		Success:       true,
		Message:       "success",
		FromAsset:     fromAsset,
		ToAsset:       toAsset,
		ExchangeRate:  exchangeRate.String(),
		Timestamp:     timestamp,
		FromPriceUsdt: fromPrice.String(),
		ToPriceUsdt:   toPrice.String(),
	}, nil
}

// getAssetPriceUSDT 获取资产对 USDT 的价格
// 对于 USD，使用 binance:tickers 的真实市场 USDT/USD 价格的倒数
// 对于 USDC，使用 binance:tickers 的真实市场 USDC/USD 价格除以 USDT/USD 价格
// 因为 fiat:tickers 中的 USDTUSD 是 C2C 场外汇率（约1.05），不是真实市场价格（约0.998）
func (l *GetExchangeRateLogic) getAssetPriceUSDT(asset string) (decimal.Decimal, bool) {
	asset = strings.ToUpper(strings.TrimSpace(asset))
	if asset == "" {
		return decimal.Zero, false
	}

	// USD 特殊处理：使用 binance:tickers 的 USDTUSD 真实市场价格
	// USD 的 USDT 价格 = 1 / USDTUSD
	if asset == "USD" {
		val, err := l.svcCtx.RedisClient.HGet(l.ctx, MarketTickerKey, "USDTUSD").Result()
		if err == nil {
			var tv TickerValue
			if json.Unmarshal([]byte(val), &tv) == nil {
				if usdtUsdRate, err := decimal.NewFromString(strings.TrimSpace(tv.Price)); err == nil && usdtUsdRate.GreaterThan(decimal.Zero) {
					// USD 的 USDT 价格 = 1 / USDTUSD
					// 例如：USDTUSD = 0.998，则 1 USD = 1/0.998 ≈ 1.002 USDT
					return decimal.NewFromInt(1).Div(usdtUsdRate), true
				}
			}
		}
		// fallback: 1 USD = 1 USDT
		return decimal.NewFromInt(1), true
	}

	// USDC 特殊处理：使用 binance:tickers 的 USDCUSD 真实市场价格
	// USDC 的 USDT 价格 = USDCUSD / USDTUSD
	if asset == "USDC" {
		var usdcUsdRate, usdtUsdRate decimal.Decimal

		// 获取 USDCUSD 价格
		if val, err := l.svcCtx.RedisClient.HGet(l.ctx, MarketTickerKey, "USDCUSD").Result(); err == nil {
			var tv TickerValue
			if json.Unmarshal([]byte(val), &tv) == nil {
				if rate, err := decimal.NewFromString(strings.TrimSpace(tv.Price)); err == nil && rate.GreaterThan(decimal.Zero) {
					usdcUsdRate = rate
				}
			}
		}

		// 获取 USDTUSD 价格
		if val, err := l.svcCtx.RedisClient.HGet(l.ctx, MarketTickerKey, "USDTUSD").Result(); err == nil {
			var tv TickerValue
			if json.Unmarshal([]byte(val), &tv) == nil {
				if rate, err := decimal.NewFromString(strings.TrimSpace(tv.Price)); err == nil && rate.GreaterThan(decimal.Zero) {
					usdtUsdRate = rate
				}
			}
		}

		// 计算 USDC/USDT = USDCUSD / USDTUSD
		if usdcUsdRate.GreaterThan(decimal.Zero) && usdtUsdRate.GreaterThan(decimal.Zero) {
			return usdcUsdRate.Div(usdtUsdRate), true
		}

		// fallback: 使用原有逻辑
		return GetAssetPrice(l.ctx, l.svcCtx.RedisClient, asset)
	}

	// 其他资产使用原有逻辑
	return GetAssetPrice(l.ctx, l.svcCtx.RedisClient, asset)
}
