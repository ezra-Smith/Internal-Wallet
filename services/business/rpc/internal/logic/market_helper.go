package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
)

// MarketTickerKey is the Redis hash key for real-time prices from market service.
const MarketTickerKey = "binance:tickers"

// FiatTickerKey is the Redis hash key for USDT->fiat mid rates from market service.
const FiatTickerKey = "fiat:tickers"

// SparklineKeyPrefix is the Redis key prefix for sparkline data.
const SparklineKeyPrefix = "binance:sparkline:"

// TickerValue represents the value stored in binance:tickers hash.
type TickerValue struct {
	Price string `json:"price"`
	Ts    int64  `json:"ts"`
}

// FiatTickerValue represents the value stored in fiat:tickers hash.
// Field: USDTCNY (asset+fiat), Value: {"buy":"...","sell":"...","mid":"...","ts":...,"provider":"..."}
type FiatTickerValue struct {
	Buy            string `json:"buy"`
	Sell           string `json:"sell"`
	Mid            string `json:"mid"`
	Ts             int64  `json:"ts"`
	Provider       string `json:"provider,omitempty"`
	CurrencySymbol string `json:"currency_symbol,omitempty"`
}

// SparklinePoint represents a single price point in sparkline ZSET.
type SparklinePoint struct {
	Price string
	Ts    int64
}

// GetAssetPrice retrieves the current USDT price for an asset from Redis.
// Returns (price_in_usdt, ok).
func GetAssetPrice(ctx context.Context, rdb *redis.Client, assetCode string) (decimal.Decimal, bool) {
	assetCode = strings.ToUpper(strings.TrimSpace(assetCode))
	if assetCode == "" {
		return decimal.Zero, false
	}
	if rdb == nil {
		return decimal.Zero, false
	}

	// Special handling for USDT: return 1 (1 USDT = 1 USDT)
	if assetCode == "USDT" {
		return decimal.NewFromInt(1), true
	}

	// Try direct pair first: e.g., BTCUSDT, USDCUSDT
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

// GetAssetPriceUSD retrieves the current USD price for an asset from Redis.
// For stablecoins (USDT, USDC), uses direct USD trading pairs (USDTUSD, USDCUSD) for accurate prices.
// For other assets, converts USDT price to USD price using USDTUSD exchange rate.
// Returns (price_in_usd, ok).
func GetAssetPriceUSD(ctx context.Context, rdb *redis.Client, assetCode string) (decimal.Decimal, bool) {
	assetCode = strings.ToUpper(strings.TrimSpace(assetCode))
	if assetCode == "" {
		return decimal.Zero, false
	}
	if rdb == nil {
		return decimal.Zero, false
	}

	// 稳定币（USDT、USDC）直接从 binance:tickers 获取 USD 交易对价格
	// 这是真实的市场价格，避免了使用 C2C 场外汇率导致的价格偏差
	if assetCode == "USDT" || assetCode == "USDC" {
		symbol := assetCode + "USD" // USDTUSD 或 USDCUSD
		val, err := rdb.HGet(ctx, MarketTickerKey, symbol).Result()
		if err == nil {
			var tv TickerValue
			if json.Unmarshal([]byte(val), &tv) == nil {
				if price, err := decimal.NewFromString(strings.TrimSpace(tv.Price)); err == nil && price.GreaterThan(decimal.Zero) {
					return price, true
				}
			}
		}
		// 如果获取 USD 交易对失败，回退到 1:1
		return decimal.NewFromInt(1), true
	}

	// 其他资产：先获取 USDT 价格
	priceUSDT, ok := GetAssetPrice(ctx, rdb, assetCode)
	if !ok {
		return decimal.Zero, false
	}

	// 使用 USDTUSD 真实市场价格转换（从 binance:tickers 获取）
	usdtUsdRate := decimal.NewFromInt(1) // 默认 1:1
	val, err := rdb.HGet(ctx, MarketTickerKey, "USDTUSD").Result()
	if err == nil {
		var tv TickerValue
		if json.Unmarshal([]byte(val), &tv) == nil {
			if rate, err := decimal.NewFromString(strings.TrimSpace(tv.Price)); err == nil && rate.GreaterThan(decimal.Zero) {
				usdtUsdRate = rate
			}
		}
	}

	// Convert USDT price to USD price
	// price_usd = price_usdt * usdt_usd_rate
	priceUSD := priceUSDT.Mul(usdtUsdRate)
	return priceUSD, true
}

// getUSDTtoUSDRate retrieves the USDT to USD exchange rate from Redis.
// Returns the rate where 1 USDT = X USD.
func getUSDTtoUSDRate(ctx context.Context, rdb *redis.Client) (decimal.Decimal, bool) {
	if rdb == nil {
		return decimal.Zero, false
	}

	fiatVal, err := rdb.HGet(ctx, FiatTickerKey, "USDTUSD").Result()
	if err != nil {
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

	return mid, true
}

// GetSparkline retrieves sparkline data (price array) for an asset from Redis ZSET.
// Returns prices ordered from oldest to newest (for chart rendering).
func GetSparkline(ctx context.Context, rdb *redis.Client, assetCode string) ([]string, error) {
	assetCode = strings.ToUpper(strings.TrimSpace(assetCode))
	if assetCode == "" || rdb == nil {
		return nil, nil
	}

	// For USDT, sparkline is always flat at 1
	if assetCode == "USDT" {
		// Return a simple flat line of "1" values
		return []string{"1", "1", "1", "1", "1"}, nil
	}

	symbol := assetCode + "USDT"
	key := SparklineKeyPrefix + symbol

	// ZRANGE key 0 -1 returns all members ordered by score (oldest to newest)
	results, err := rdb.ZRangeWithScores(ctx, key, 0, -1).Result()
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, nil
	}

	prices := make([]string, 0, len(results))
	for _, z := range results {
		price, ok := z.Member.(string)
		if !ok {
			continue
		}
		prices = append(prices, price)
	}

	return prices, nil
}

// GetSparklineLast retrieves sparkline data for the last N hours.
func GetSparklineLast(ctx context.Context, rdb *redis.Client, assetCode string, hours int) ([]string, error) {
	assetCode = strings.ToUpper(strings.TrimSpace(assetCode))
	if assetCode == "" || rdb == nil {
		return nil, nil
	}

	if assetCode == "USDT" {
		return []string{"1", "1", "1", "1", "1"}, nil
	}

	symbol := assetCode + "USDT"
	key := SparklineKeyPrefix + symbol

	now := time.Now().UnixMilli()
	startMs := now - int64(hours)*60*60*1000

	results, err := rdb.ZRangeByScoreWithScores(ctx, key, &redis.ZRangeBy{
		Min: formatInt64(startMs),
		Max: "+inf",
	}).Result()
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, nil
	}

	prices := make([]string, 0, len(results))
	for _, z := range results {
		price, ok := z.Member.(string)
		if !ok {
			continue
		}
		prices = append(prices, price)
	}

	return prices, nil
}

// Calculate24hPriceChange calculates the 24h price change percentage.
// Returns (changePercent, ok).
func Calculate24hPriceChange(sparkline []string) (string, bool) {
	if len(sparkline) < 2 {
		return "0", false
	}

	// First price is oldest (24h ago), last is current
	oldPriceStr := sparkline[0]
	newPriceStr := sparkline[len(sparkline)-1]

	oldPrice, err1 := decimal.NewFromString(oldPriceStr)
	newPrice, err2 := decimal.NewFromString(newPriceStr)

	if err1 != nil || err2 != nil || oldPrice.IsZero() {
		return "0", false
	}

	// Change = ((new - old) / old) * 100
	change := newPrice.Sub(oldPrice).Div(oldPrice).Mul(decimal.NewFromInt(100))

	// Round to 2 decimal places
	return change.Round(2).String(), true
}

// BatchGetAssetPrices retrieves prices for multiple assets at once.
func BatchGetAssetPrices(ctx context.Context, rdb *redis.Client, assetCodes []string) map[string]decimal.Decimal {
	result := make(map[string]decimal.Decimal, len(assetCodes))
	if rdb == nil || len(assetCodes) == 0 {
		return result
	}

	// Build symbols list (crypto) and fiat fields list (USDT->fiat)
	symbols := make([]string, 0, len(assetCodes))
	fiatFields := make([]string, 0, len(assetCodes))
	fiatFieldToCode := make(map[string]string, len(assetCodes))
	for _, code := range assetCodes {
		code = strings.ToUpper(strings.TrimSpace(code))
		if code == "" {
			continue
		}
		if code == "USDT" {
			result[code] = decimal.NewFromInt(1)
			continue
		}
		symbols = append(symbols, code+"USDT")
		fiatField := "USDT" + code
		fiatFields = append(fiatFields, fiatField)
		fiatFieldToCode[fiatField] = code
	}

	if len(symbols) == 0 && len(fiatFields) == 0 {
		return result
	}

	if len(symbols) > 0 {
		// HMGET for crypto prices
		vals, err := rdb.HMGet(ctx, MarketTickerKey, symbols...).Result()
		if err == nil {
			for i, val := range vals {
				if val == nil {
					continue
				}
				valStr, ok := val.(string)
				if !ok {
					continue
				}

				var tv TickerValue
				if err := json.Unmarshal([]byte(valStr), &tv); err != nil {
					continue
				}

				price, err := decimal.NewFromString(strings.TrimSpace(tv.Price))
				if err != nil || !price.GreaterThan(decimal.Zero) {
					continue
				}

				// Extract asset code from symbol (e.g., BTCUSDT -> BTC)
				symbol := symbols[i]
				assetCode := strings.TrimSuffix(symbol, "USDT")
				result[assetCode] = price
			}
		}
	}

	if len(fiatFields) > 0 {
		// HMGET for fiat mid rates (USDT->fiat), then invert to get fiat->USDT.
		vals, err := rdb.HMGet(ctx, FiatTickerKey, fiatFields...).Result()
		if err == nil {
			for i, val := range vals {
				if val == nil {
					continue
				}
				valStr, ok := val.(string)
				if !ok {
					continue
				}

				var ftv FiatTickerValue
				if err := json.Unmarshal([]byte(valStr), &ftv); err != nil {
					continue
				}

				mid, err := decimal.NewFromString(strings.TrimSpace(ftv.Mid))
				if err != nil || !mid.GreaterThan(decimal.Zero) {
					continue
				}

				code := fiatFieldToCode[fiatFields[i]]
				if code == "" {
					continue
				}
				if _, exists := result[code]; exists {
					continue
				}
				result[code] = decimal.NewFromInt(1).Div(mid)
			}
		}
	}

	return result
}

func formatInt64(n int64) string {
	return decimal.NewFromInt(n).String()
}

// HourlySparklinePoint represents a price point aggregated by hour.
type HourlySparklinePoint struct {
	Price     decimal.Decimal
	Timestamp time.Time // Hour timestamp
}

// GetHourlySparkline retrieves sparkline data aggregated by hour (last 24 hours).
// Market service samples every 5 minutes, so we aggregate by hour.
// Returns hourly points ordered from oldest to newest.
func GetHourlySparkline(ctx context.Context, rdb *redis.Client, assetCode string, hours int) ([]HourlySparklinePoint, error) {
	assetCode = strings.ToUpper(strings.TrimSpace(assetCode))
	if assetCode == "" || rdb == nil {
		return nil, nil
	}

	if assetCode == "USDT" {
		// Return flat line for USDT
		now := time.Now()
		points := make([]HourlySparklinePoint, hours)
		for i := 0; i < hours; i++ {
			points[i] = HourlySparklinePoint{
				Price:     decimal.NewFromInt(1),
				Timestamp: now.Add(-time.Duration(hours-i-1) * time.Hour).Truncate(time.Hour),
			}
		}
		return points, nil
	}

	symbol := assetCode + "USDT"
	key := SparklineKeyPrefix + symbol

	now := time.Now()
	startMs := now.Add(-time.Duration(hours) * time.Hour).UnixMilli()

	// Get all data points in the last N hours
	results, err := rdb.ZRangeByScoreWithScores(ctx, key, &redis.ZRangeBy{
		Min: formatInt64(startMs),
		Max: "+inf",
	}).Result()
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, nil
	}

	// Aggregate by hour: group data points by hour, take the last price of each hour
	hourlyMap := make(map[int64]struct {
		Price     decimal.Decimal
		Timestamp time.Time
	}) // hour timestamp (ms) -> {price, original timestamp}

	for _, z := range results {
		priceStr, ok := z.Member.(string)
		if !ok {
			continue
		}
		price, err := decimal.NewFromString(strings.TrimSpace(priceStr))
		if err != nil || !price.GreaterThan(decimal.Zero) {
			continue
		}

		// Get the hour timestamp for this data point
		timestamp := time.UnixMilli(int64(z.Score))
		hourTimestamp := timestamp.Truncate(time.Hour).UnixMilli()

		// Keep the last price for each hour (later timestamps overwrite earlier ones)
		if existing, exists := hourlyMap[hourTimestamp]; !exists || timestamp.After(existing.Timestamp) {
			hourlyMap[hourTimestamp] = struct {
				Price     decimal.Decimal
				Timestamp time.Time
			}{Price: price, Timestamp: timestamp}
		}
	}

	// Convert to sorted slice
	points := make([]HourlySparklinePoint, 0, len(hourlyMap))
	for hourMs, data := range hourlyMap {
		points = append(points, HourlySparklinePoint{
			Price:     data.Price,
			Timestamp: time.UnixMilli(hourMs),
		})
	}

	// Sort by timestamp (oldest first)
	for i := 0; i < len(points)-1; i++ {
		minIdx := i
		for j := i + 1; j < len(points); j++ {
			if points[j].Timestamp.Before(points[minIdx].Timestamp) {
				minIdx = j
			}
		}
		if minIdx != i {
			points[i], points[minIdx] = points[minIdx], points[i]
		}
	}

	return points, nil
}

// GenerateSVGSparkline generates an SVG sparkline chart from hourly price data.
// width and height are in pixels (default: 60x20 for mini charts).
// Returns SVG as string, and price change percentage.
func GenerateSVGSparkline(points []HourlySparklinePoint, width, height int) (string, string) {
	if len(points) < 2 {
		// Return empty SVG for insufficient data
		return `<svg width="60" height="20" xmlns="http://www.w3.org/2000/svg"><path d="M0,10 L60,10" stroke="#999" stroke-width="1" fill="none"/></svg>`, "0"
	}

	if width <= 0 {
		width = 60
	}
	if height <= 0 {
		height = 20
	}

	// Calculate min and max prices for scaling
	minPrice := points[0].Price
	maxPrice := points[0].Price
	for _, p := range points {
		if p.Price.LessThan(minPrice) {
			minPrice = p.Price
		}
		if p.Price.GreaterThan(maxPrice) {
			maxPrice = p.Price
		}
	}

	// Calculate price change percentage
	firstPrice := points[0].Price
	lastPrice := points[len(points)-1].Price
	var changePercent string
	if firstPrice.IsZero() {
		changePercent = "0"
	} else {
		change := lastPrice.Sub(firstPrice).Div(firstPrice).Mul(decimal.NewFromInt(100))
		changePercent = change.Round(2).String()
	}

	// Padding for better visualization
	padding := 2
	chartWidth := width - padding*2
	chartHeight := height - padding*2

	// Calculate scale (avoid division by zero)
	priceRange := maxPrice.Sub(minPrice)
	priceRangeFloat := priceRange.InexactFloat64()
	if priceRangeFloat == 0 {
		priceRangeFloat = 1.0
		// If all prices are the same, center the line
		minPrice = minPrice.Sub(decimal.NewFromInt(1))
		maxPrice = maxPrice.Add(decimal.NewFromInt(1))
		priceRangeFloat = 2.0
	}

	// Build path data
	pathParts := make([]string, 0, len(points))
	for i, p := range points {
		x := float64(padding) + float64(chartWidth)*float64(i)/float64(max(1, len(points)-1))
		// Invert Y axis (SVG Y increases downward, we want higher prices at top)
		priceOffset := p.Price.Sub(minPrice).InexactFloat64()
		normalizedY := priceOffset / priceRangeFloat
		// Clamp normalizedY to [0, 1] to avoid out-of-bounds
		if normalizedY < 0 {
			normalizedY = 0
		}
		if normalizedY > 1 {
			normalizedY = 1
		}
		y := float64(padding) + float64(chartHeight)*(1.0-normalizedY)

		if i == 0 {
			pathParts = append(pathParts, fmt.Sprintf("M%.2f,%.2f", x, y))
		} else {
			pathParts = append(pathParts, fmt.Sprintf("L%.2f,%.2f", x, y))
		}
	}
	pathData := strings.Join(pathParts, " ")

	// Use neutral gray color - frontend will determine green/red based on priceChangePercent
	// This allows frontend to apply custom styling (e.g., gradient fills, different stroke widths)
	strokeColor := "#999" // Neutral gray - frontend will apply color based on changePercent

	// Generate SVG (frontend will apply color based on priceChangePercent)
	svg := fmt.Sprintf(`<svg width="%d" height="%d" xmlns="http://www.w3.org/2000/svg"><path d="%s" stroke="%s" stroke-width="1.5" fill="none" stroke-linecap="round" stroke-linejoin="round"/></svg>`,
		width, height, pathData, strokeColor)

	return svg, changePercent
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
