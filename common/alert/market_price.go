package alert

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
)

// MarketTickerKey is the Redis hash key for real-time prices from market service.
const MarketTickerKey = "binance:tickers"

// FiatTickerKey is the Redis hash key for USDT->fiat mid rates from market service.
const FiatTickerKey = "fiat:tickers"

// TickerValue represents the value stored in binance:tickers hash.
type TickerValue struct {
	Price string `json:"price"`
	Ts    int64  `json:"ts"`
}

// FiatTickerValue represents the value stored in fiat:tickers hash.
type FiatTickerValue struct {
	Buy            string `json:"buy"`
	Sell           string `json:"sell"`
	Mid            string `json:"mid"`
	Ts             int64  `json:"ts"`
	Provider       string `json:"provider,omitempty"`
	CurrencySymbol string `json:"currency_symbol,omitempty"`
}

// GetAssetPrice retrieves the current USDT price for an asset from Redis.
// Returns (price, ok). If asset is USDT, returns 1.
func GetAssetPrice(ctx context.Context, rdb *redis.Client, assetCode string) (decimal.Decimal, bool) {
	assetCode = strings.ToUpper(strings.TrimSpace(assetCode))
	if assetCode == "" {
		return decimal.Zero, false
	}
	if assetCode == "USDT" {
		return decimal.NewFromInt(1), true
	}
	if rdb == nil {
		return decimal.Zero, false
	}

	// Try direct pair first: e.g., BTCUSDT
	symbol := assetCode + "USDT"
	val, err := rdb.HGet(ctx, MarketTickerKey, symbol).Result()
	if err == nil {
		var tv TickerValue
		if err := json.Unmarshal([]byte(val), &tv); err != nil {
			return decimal.Zero, false
		}

		price, err := decimal.NewFromString(strings.TrimSpace(tv.Price))
		if err != nil || !price.GreaterThan(decimal.Zero) {
			return decimal.Zero, false
		}

		return price, true
	}

	// Fallback: treat assetCode as a fiat currency and invert USDT->fiat mid rate.
	// Example: USDTCNY mid=7.2 means 1 CNY = 1/7.2 USDT.
	// This handles cases like CNY, USD, EUR, etc.
	fiatField := "USDT" + assetCode
	fiatVal, err := rdb.HGet(ctx, FiatTickerKey, fiatField).Result()
	if err != nil {
		// If both crypto and fiat lookups fail, return false
		return decimal.Zero, false
	}

	var ftv FiatTickerValue
	if err := json.Unmarshal([]byte(fiatVal), &ftv); err != nil {
		return decimal.Zero, false
	}

	mid, err := decimal.NewFromString(strings.TrimSpace(ftv.Mid))
	if err != nil || !mid.GreaterThan(decimal.Zero) {
		return decimal.Zero, false
	}

	return decimal.NewFromInt(1).Div(mid), true
}

// ConvertToUSD 将金额转换为 USD（便捷方法）
func ConvertToUSD(ctx context.Context, rdb *redis.Client, assetCode string, amount decimal.Decimal) (decimal.Decimal, bool) {
	if assetCode == "USD" || assetCode == "USDT" {
		return amount, true
	}

	price, ok := GetAssetPrice(ctx, rdb, assetCode)
	if !ok {
		return decimal.Zero, false
	}

	return amount.Mul(price), true
}
