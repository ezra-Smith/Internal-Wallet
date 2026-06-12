package logic

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
)

const fiatTickerKey = "fiat:tickers"
const usdtUsdField = "USDTUSD"

type fiatTickerValue struct {
	Mid string `json:"mid"`
}

func getUsdtUsdRate(ctx context.Context, rdb *redis.Client) (decimal.Decimal, bool) {
	if rdb == nil {
		return decimal.Zero, false
	}
	val, err := rdb.HGet(ctx, fiatTickerKey, usdtUsdField).Result()
	if err != nil {
		return decimal.Zero, false
	}
	var tv fiatTickerValue
	if err := json.Unmarshal([]byte(val), &tv); err != nil {
		return decimal.Zero, false
	}
	midStr := strings.TrimSpace(tv.Mid)
	if midStr == "" {
		return decimal.Zero, false
	}
	mid, err := decimal.NewFromString(midStr)
	if err != nil || !mid.GreaterThan(decimal.Zero) {
		return decimal.Zero, false
	}
	return mid, true
}

func calcUsdByRate(amount string, rate decimal.Decimal) (string, bool) {
	amount = strings.TrimSpace(amount)
	if amount == "" || !rate.GreaterThan(decimal.Zero) {
		return "", false
	}
	amt, err := decimal.NewFromString(amount)
	if err != nil || !amt.GreaterThan(decimal.Zero) {
		return "", false
	}
	usd := amt.Mul(rate)
	return usd.StringFixed(2), true
}

func calcTransferBatchUsd(ctx context.Context, rdb *redis.Client, currency string, totalAmount string) (string, bool) {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if currency != "USDT" {
		return "", false
	}
	rate, ok := getUsdtUsdRate(ctx, rdb)
	if !ok {
		return "", false
	}
	return calcUsdByRate(totalAmount, rate)
}
