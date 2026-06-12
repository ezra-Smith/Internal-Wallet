package binance

import (
	"context"
	"strconv"
	"sync"
	"time"

	"internalwallet/services/market/rpc/internal/config"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
)

// SparklinePoint represents a single price point in a sparkline.
// Stored in Redis ZSET with score=timestamp(ms), member=price.
type SparklinePoint struct {
	Price string `json:"price"` // Price as string to preserve precision
	Ts    int64  `json:"ts"`    // Unix milliseconds (used as ZSET score)
}

// SparklineCollector samples price data periodically to build mini charts (sparklines).
// Uses Redis ZSET for efficient time-range queries:
//   - Score: Unix timestamp in milliseconds
//   - Member: Price string (e.g., "94250.00")
//
// This allows:
//   - ZRANGEBYSCORE for "get last N hours"
//   - ZREMRANGEBYSCORE for cleanup old data
//   - Automatic deduplication by timestamp
type SparklineCollector struct {
	cfg   config.Config
	redis *redis.Client

	// mu protects latestPrices
	mu           sync.RWMutex
	latestPrices map[string]SparklinePoint // symbol -> latest price point
}

// NewSparklineCollector creates a new sparkline data collector.
func NewSparklineCollector(cfg config.Config, redisClient *redis.Client) *SparklineCollector {
	return &SparklineCollector{
		cfg:          cfg,
		redis:        redisClient,
		latestPrices: make(map[string]SparklinePoint),
	}
}

// UpdatePrices updates the latest prices from ticker events.
// Called by the main streamer when new ticker data arrives.
func (c *SparklineCollector) UpdatePrices(events []miniTickerEvent) {
	if len(events) == 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	for _, e := range events {
		if len(e.Symbol) == 0 || len(e.Close) == 0 || e.EventTime <= 0 {
			continue
		}
		c.latestPrices[e.Symbol] = SparklinePoint{
			Price: e.Close,
			Ts:    e.EventTime,
		}
	}
}

// Run starts the sparkline sampling loop.
// It periodically samples the latest prices and stores them in Redis ZSET.
func (c *SparklineCollector) Run(ctx context.Context) {
	if !c.cfg.Sparkline.Enabled {
		logx.Info("Sparkline collection disabled")
		return
	}

	interval := c.cfg.SparklineSampleInterval()
	retentionMs := c.cfg.SparklineRetentionMs()
	keyPrefix := c.cfg.Redis.SparklineKeyPrefix

	logx.Infof("Sparkline collector started: interval=%s, retention=%s, keyPrefix=%s",
		interval, time.Duration(retentionMs)*time.Millisecond, keyPrefix)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logx.Info("Sparkline collector stopped")
			return
		case <-ticker.C:
			c.sample(ctx, keyPrefix, retentionMs)
		}
	}
}

// sample takes a snapshot of current prices and stores them in Redis ZSET.
// Uses ZADD with score=timestamp, then ZREMRANGEBYSCORE to clean old data.
func (c *SparklineCollector) sample(ctx context.Context, keyPrefix string, retentionMs int64) {
	c.mu.RLock()
	snapshot := make(map[string]SparklinePoint, len(c.latestPrices))
	for symbol, point := range c.latestPrices {
		snapshot[symbol] = point
	}
	c.mu.RUnlock()

	if len(snapshot) == 0 {
		return
	}

	now := time.Now().UnixMilli()
	cutoff := now - retentionMs // Remove data older than this
	pipe := c.redis.Pipeline()
	count := 0

	for symbol, point := range snapshot {
		key := keyPrefix + symbol

		// ZADD: score=timestamp(ms), member=price
		// Using NX would prevent updates, but we want the latest price at each sample time
		pipe.ZAdd(ctx, key, redis.Z{
			Score:  float64(now),
			Member: point.Price,
		})

		// Remove data older than retention period
		// ZREMRANGEBYSCORE key -inf (now - retention)
		pipe.ZRemRangeByScore(ctx, key, "-inf", floatToString(float64(cutoff)))

		count++
	}

	if count == 0 {
		return
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		if ctx.Err() == nil {
			logx.Errorf("Sparkline Redis write failed: %v", err)
		}
		return
	}

	logx.Debugf("Sparkline sampled %d symbols", count)
}

// floatToString converts float64 to string for Redis commands.
func floatToString(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// GetSparkline retrieves sparkline data for a symbol from Redis ZSET.
// Returns all points ordered by timestamp (oldest to newest).
func GetSparkline(ctx context.Context, rdb *redis.Client, keyPrefix, symbol string) ([]SparklinePoint, error) {
	key := keyPrefix + symbol

	// ZRANGE key 0 -1 WITHSCORES returns all members with scores
	results, err := rdb.ZRangeWithScores(ctx, key, 0, -1).Result()
	if err != nil {
		return nil, err
	}

	points := make([]SparklinePoint, 0, len(results))
	for _, z := range results {
		price, ok := z.Member.(string)
		if !ok {
			continue
		}
		points = append(points, SparklinePoint{
			Price: price,
			Ts:    int64(z.Score),
		})
	}

	return points, nil
}

// GetSparklineByTimeRange retrieves sparkline data within a time range.
// startMs and endMs are Unix timestamps in milliseconds.
// Use 0 for startMs to get from the beginning, or -1 for endMs to get until now.
func GetSparklineByTimeRange(ctx context.Context, rdb *redis.Client, keyPrefix, symbol string, startMs, endMs int64) ([]SparklinePoint, error) {
	key := keyPrefix + symbol

	minScore := "-inf"
	maxScore := "+inf"

	if startMs > 0 {
		minScore = floatToString(float64(startMs))
	}
	if endMs > 0 {
		maxScore = floatToString(float64(endMs))
	}

	// ZRANGEBYSCORE key min max WITHSCORES
	results, err := rdb.ZRangeByScoreWithScores(ctx, key, &redis.ZRangeBy{
		Min: minScore,
		Max: maxScore,
	}).Result()
	if err != nil {
		return nil, err
	}

	points := make([]SparklinePoint, 0, len(results))
	for _, z := range results {
		price, ok := z.Member.(string)
		if !ok {
			continue
		}
		points = append(points, SparklinePoint{
			Price: price,
			Ts:    int64(z.Score),
		})
	}

	return points, nil
}

// GetSparklineLast retrieves the last N hours of sparkline data.
func GetSparklineLast(ctx context.Context, rdb *redis.Client, keyPrefix, symbol string, hours int) ([]SparklinePoint, error) {
	now := time.Now().UnixMilli()
	startMs := now - int64(hours)*60*60*1000
	return GetSparklineByTimeRange(ctx, rdb, keyPrefix, symbol, startMs, now)
}

// GetSparklinePricesOnly retrieves just the price values for a symbol (for simple charts).
// Returns a slice of price strings, ordered from oldest to newest.
func GetSparklinePricesOnly(ctx context.Context, rdb *redis.Client, keyPrefix, symbol string) ([]string, error) {
	points, err := GetSparkline(ctx, rdb, keyPrefix, symbol)
	if err != nil {
		return nil, err
	}

	prices := make([]string, len(points))
	for i, p := range points {
		prices[i] = p.Price
	}

	return prices, nil
}

// GetSparklineLastPricesOnly retrieves just the price values for the last N hours.
func GetSparklineLastPricesOnly(ctx context.Context, rdb *redis.Client, keyPrefix, symbol string, hours int) ([]string, error) {
	points, err := GetSparklineLast(ctx, rdb, keyPrefix, symbol, hours)
	if err != nil {
		return nil, err
	}

	prices := make([]string, len(points))
	for i, p := range points {
		prices[i] = p.Price
	}

	return prices, nil
}
