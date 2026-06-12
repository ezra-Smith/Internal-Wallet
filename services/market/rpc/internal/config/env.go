package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

// ApplyEnvOverrides overrides config values with env vars (if present).
//
// This repo generally uses YAML config files, but some deployments prefer
// env vars for secrets and per-environment overrides.
func ApplyEnvOverrides(c *Config) error {
	if c == nil {
		return nil
	}

	// WebSocket
	if v := strings.TrimSpace(os.Getenv("MARKET_BINANCE_WS_BASE_URL")); v != "" {
		c.Binance.BaseURL = v
	}
	if v := strings.TrimSpace(os.Getenv("MARKET_BINANCE_WS_ENDPOINT")); v != "" {
		c.Binance.Endpoint = v
	}

	// Redis connection (compatible with existing CacheRedis pattern)
	if v := strings.TrimSpace(os.Getenv("MARKET_REDIS_ADDR")); v != "" {
		if len(c.CacheRedis) == 0 {
			c.CacheRedis = cache.CacheConf{{RedisConf: redis.RedisConf{Host: v, Type: "node"}}}
		} else {
			c.CacheRedis[0].Host = v
		}
	}
	if v := strings.TrimSpace(os.Getenv("MARKET_REDIS_PASS")); v != "" {
		if len(c.CacheRedis) == 0 {
			c.CacheRedis = cache.CacheConf{{RedisConf: redis.RedisConf{Host: "localhost:6379", Pass: v, Type: "node"}}}
		} else {
			c.CacheRedis[0].Pass = v
		}
	}
	if v := strings.TrimSpace(os.Getenv("MARKET_REDIS_DB")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("invalid MARKET_REDIS_DB: %w", err)
		}
		c.Redis.DB = n
	}
	if v := strings.TrimSpace(os.Getenv("MARKET_REDIS_TICKER_HASH_KEY")); v != "" {
		c.Redis.TickerHashKey = v
	}
	if v := strings.TrimSpace(os.Getenv("MARKET_REDIS_TTL_SECONDS")); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid MARKET_REDIS_TTL_SECONDS: %w", err)
		}
		c.Redis.TTLSeconds = n
	}

	// Reconnect backoff
	if v := strings.TrimSpace(os.Getenv("MARKET_RECONNECT_INITIAL_BACKOFF_MS")); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid MARKET_RECONNECT_INITIAL_BACKOFF_MS: %w", err)
		}
		c.Reconnect.InitialBackoffMillis = n
	}
	if v := strings.TrimSpace(os.Getenv("MARKET_RECONNECT_MAX_BACKOFF_MS")); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid MARKET_RECONNECT_MAX_BACKOFF_MS: %w", err)
		}
		c.Reconnect.MaxBackoffMillis = n
	}
	if v := strings.TrimSpace(os.Getenv("MARKET_RECONNECT_MULTIPLIER")); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return fmt.Errorf("invalid MARKET_RECONNECT_MULTIPLIER: %w", err)
		}
		c.Reconnect.Multiplier = f
	}
	if v := strings.TrimSpace(os.Getenv("MARKET_RECONNECT_JITTER")); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return fmt.Errorf("invalid MARKET_RECONNECT_JITTER: %w", err)
		}
		c.Reconnect.Jitter = f
	}

	// Health
	if v := strings.TrimSpace(os.Getenv("MARKET_HEALTH_ENABLED")); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("invalid MARKET_HEALTH_ENABLED: %w", err)
		}
		c.Health.Enabled = b
	}
	if v := strings.TrimSpace(os.Getenv("MARKET_HEALTH_LISTEN_ON")); v != "" {
		c.Health.ListenOn = v
	}
	if v := strings.TrimSpace(os.Getenv("MARKET_HEALTH_PATH")); v != "" {
		c.Health.Path = v
	}
	if v := strings.TrimSpace(os.Getenv("MARKET_HEALTH_STALE_AFTER_MS")); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid MARKET_HEALTH_STALE_AFTER_MS: %w", err)
		}
		c.Health.StaleAfterMillis = n
	}

	// Sparkline
	if v := strings.TrimSpace(os.Getenv("MARKET_SPARKLINE_ENABLED")); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("invalid MARKET_SPARKLINE_ENABLED: %w", err)
		}
		c.Sparkline.Enabled = b
	}
	if v := strings.TrimSpace(os.Getenv("MARKET_SPARKLINE_SAMPLE_INTERVAL_SECONDS")); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid MARKET_SPARKLINE_SAMPLE_INTERVAL_SECONDS: %w", err)
		}
		c.Sparkline.SampleIntervalSeconds = n
	}
	if v := strings.TrimSpace(os.Getenv("MARKET_SPARKLINE_RETENTION_HOURS")); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid MARKET_SPARKLINE_RETENTION_HOURS: %w", err)
		}
		c.Sparkline.RetentionHours = n
	}
	if v := strings.TrimSpace(os.Getenv("MARKET_REDIS_SPARKLINE_KEY_PREFIX")); v != "" {
		c.Redis.SparklineKeyPrefix = v
	}

	// Fiat (USDT -> fiat)
	if v := strings.TrimSpace(os.Getenv("MARKET_FIAT_ENABLED")); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("invalid MARKET_FIAT_ENABLED: %w", err)
		}
		c.Fiat.Enabled = b
	}
	if v := strings.TrimSpace(os.Getenv("MARKET_FIAT_UPDATE_INTERVAL_SECONDS")); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid MARKET_FIAT_UPDATE_INTERVAL_SECONDS: %w", err)
		}
		c.Fiat.UpdateIntervalSeconds = n
	}
	if v := strings.TrimSpace(os.Getenv("MARKET_FIAT_REQUEST_TIMEOUT_MILLIS")); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid MARKET_FIAT_REQUEST_TIMEOUT_MILLIS: %w", err)
		}
		c.Fiat.RequestTimeoutMillis = n
	}
	if v := strings.TrimSpace(os.Getenv("MARKET_FIAT_MAX_RETRIES")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("invalid MARKET_FIAT_MAX_RETRIES: %w", err)
		}
		c.Fiat.MaxRetries = n
	}
	if v := strings.TrimSpace(os.Getenv("MARKET_FIAT_BASE_URL")); v != "" {
		c.Fiat.BaseURL = v
	}
	if v := strings.TrimSpace(os.Getenv("MARKET_FIAT_ENDPOINT")); v != "" {
		c.Fiat.Endpoint = v
	}
	if v := strings.TrimSpace(os.Getenv("MARKET_FIAT_REDIS_HASH_KEY")); v != "" {
		c.Fiat.RedisHashKey = v
	}
	if v := strings.TrimSpace(os.Getenv("MARKET_FIAT_TTL_SECONDS")); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid MARKET_FIAT_TTL_SECONDS: %w", err)
		}
		c.Fiat.TTLSeconds = n
	}

	return nil
}
