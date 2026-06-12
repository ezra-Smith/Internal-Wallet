package fiat

import (
	"context"
	"strconv"
	"strings"
	"time"

	"internalwallet/services/market/rpc/internal/config"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
)

// Collector periodically fetches USDT->fiat mid rates and stores them in Redis.
//
// Storage:
//   - Hash key: cfg.Fiat.RedisHashKey (default: fiat:tickers)
//   - Field:    {ASSET}{FIAT} (e.g., USDTCNY)
//   - Value:    JSON {"buy":"...","sell":"...","mid":"...","ts":1734567890123,"provider":"binance_c2c","currency_symbol":"¥"}
type Collector struct {
	cfg    config.Config
	redis  *redis.Client
	client *C2CClient
}

func NewCollector(cfg config.Config, redisClient *redis.Client) *Collector {
	c := NewC2CClient(cfg.Fiat.BaseURL, cfg.Fiat.Endpoint, cfg.FiatRequestTimeout(), cfg.Fiat.MaxRetries)
	return &Collector{
		cfg:    cfg,
		redis:  redisClient,
		client: c,
	}
}

func (c *Collector) Run(ctx context.Context) {
	if !c.cfg.Fiat.Enabled {
		logx.Info("Fiat collection disabled")
		return
	}
	if c.redis == nil {
		logx.Error("Fiat collector disabled: redis client is nil")
		return
	}
	if len(c.cfg.Fiat.Pairs) == 0 {
		logx.Infof("Fiat collector enabled but no pairs configured")
		return
	}

	interval := c.cfg.FiatUpdateInterval()
	ttl := c.cfg.FiatRedisTTL()
	hashKey := strings.TrimSpace(c.cfg.Fiat.RedisHashKey)

	logx.Infof("Fiat collector started: interval=%s, ttl=%s, hashKey=%s, pairs=%d", interval, ttl, hashKey, len(c.cfg.Fiat.Pairs))

	// Fetch once immediately on startup.
	c.fetchAndStore(ctx, hashKey, ttl)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logx.Info("Fiat collector stopped")
			return
		case <-ticker.C:
			c.fetchAndStore(ctx, hashKey, ttl)
		}
	}
}

func (c *Collector) fetchAndStore(ctx context.Context, hashKey string, ttl time.Duration) {
	updates := make(map[string]string)
	success := 0
	total := 0

	for _, p := range c.cfg.Fiat.Pairs {
		if !p.Enabled {
			continue
		}
		asset := strings.ToUpper(strings.TrimSpace(p.Asset))
		fiatCurrency := strings.ToUpper(strings.TrimSpace(p.FiatCurrency))
		if asset == "" || fiatCurrency == "" {
			continue
		}
		total++

		quote, err := c.client.GetBothPrices(ctx, asset, fiatCurrency)
		if err != nil {
			if ctx.Err() == nil {
				logx.Errorf("Fiat fetch failed for %s/%s: %v", asset, fiatCurrency, err)
			}
			continue
		}

		field := asset + fiatCurrency
		updates[field] = encodeFiatTickerValue(quote)
		success++
	}

	if len(updates) == 0 {
		if total > 0 && ctx.Err() == nil {
			logx.Infof("Fiat collector: no updates (success=%d total=%d)", success, total)
		}
		return
	}

	pipe := c.redis.TxPipeline()
	pipe.HSet(ctx, hashKey, updates)
	if ttl > 0 {
		pipe.Expire(ctx, hashKey, ttl)
	}
	_, err := pipe.Exec(ctx)
	if err != nil {
		if ctx.Err() == nil {
			logx.Errorf("Fiat Redis write failed: %v", err)
		}
		return
	}

	logx.Infof("Fiat rates updated: success=%d total=%d", success, total)
}

func encodeFiatTickerValue(q *FiatQuote) string {
	// Value format:
	// {"buy":"...","sell":"...","mid":"...","ts":<unixMs>,"provider":"...","currency_symbol":"..."}
	b := make([]byte, 0, 160)
	b = append(b, `{"buy":`...)
	b = strconv.AppendQuote(b, q.BuyPrice.String())
	b = append(b, `,"sell":`...)
	b = strconv.AppendQuote(b, q.SellPrice.String())
	b = append(b, `,"mid":`...)
	b = strconv.AppendQuote(b, q.MidPrice.String())
	b = append(b, `,"ts":`...)
	b = strconv.AppendInt(b, q.TimestampMs, 10)
	b = append(b, `,"provider":`...)
	b = strconv.AppendQuote(b, q.Provider)
	if strings.TrimSpace(q.CurrencySymbol) != "" {
		b = append(b, `,"currency_symbol":`...)
		b = strconv.AppendQuote(b, q.CurrencySymbol)
	}
	b = append(b, '}')
	return string(b)
}
