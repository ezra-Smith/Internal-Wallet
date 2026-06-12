package config

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/core/stores/cache"
)

type Config struct {
	service.ServiceConf

	// Enabled controls whether external collectors (Binance WS / Fiat HTTP) are active.
	// In restricted egress environments, set Enabled=false to run health-only mode.
	Enabled bool `json:",default=true"`

	// CacheRedis follows the existing microservice pattern in this repo (go-zero cache conf).
	// Only the first node entry is used by this service.
	CacheRedis cache.CacheConf `json:",optional"`

	Redis     RedisConfig     `json:"Redis"`
	Binance   BinanceConfig   `json:"Binance"`
	Reconnect ReconnectConfig `json:"Reconnect"`
	Health    HealthConfig    `json:"Health,optional"`
	Sparkline SparklineConfig `json:"Sparkline,optional"`
	Fiat      FiatConfig      `json:"Fiat,optional"`
}

type RedisConfig struct {
	DB int `json:",default=0"`

	// TickerHashKey stores per-symbol mini-ticker values.
	// Example:
	//   HGET binance:tickers BTCUSDT -> {"price":"65000.12","ts":1734567890123}
	TickerHashKey string `json:",default=binance:tickers"`

	// TTLSeconds sets an expire time on the hash key; 0 disables TTL.
	TTLSeconds int64 `json:",default=30"`

	// SparklineKeyPrefix is the prefix for sparkline data lists.
	// Each symbol gets its own list: binance:sparkline:BTCUSDT
	SparklineKeyPrefix string `json:",default=binance:sparkline:"`

	// Client options (reasonable production defaults; override if needed).
	DialTimeoutMillis  int64 `json:",default=5000"`
	ReadTimeoutMillis  int64 `json:",default=3000"`
	WriteTimeoutMillis int64 `json:",default=3000"`
	PoolSize           int   `json:",default=50"`
	MinIdleConns       int   `json:",default=10"`
	MaxRetries         int   `json:",default=3"`
}

type BinanceConfig struct {
	// BaseURL must be one of:
	//   wss://stream.binance.com:9443
	//   wss://stream.binance.com:443
	BaseURL string `json:",default=wss://stream.binance.com:9443"`

	// Endpoint should be: /ws/!miniTicker@arr
	Endpoint string `json:",default=/ws/!miniTicker@arr"`

	HandshakeTimeoutMillis int64 `json:",default=10000"`
	ReadTimeoutMillis      int64 `json:",default=90000"`
	WriteTimeoutMillis     int64 `json:",default=10000"`

	// MaxMessageBytes is a safety limit for the all-market payload.
	MaxMessageBytes int64 `json:",default=16777216"` // 16 MiB

	// MaxConnAgeSeconds forces a graceful reconnect before Binance's 24h session expiry.
	MaxConnAgeSeconds int64 `json:",default=86340"` // 23h 59m
}

type ReconnectConfig struct {
	InitialBackoffMillis int64   `json:",default=500"`
	MaxBackoffMillis     int64   `json:",default=30000"`
	Multiplier           float64 `json:",default=2"`
	Jitter               float64 `json:",default=0.2"` // 0.0 ~ 1.0
}

type HealthConfig struct {
	Enabled  bool   `json:",default=true"`
	ListenOn string `json:",default=0.0.0.0:18080"`
	Path     string `json:",default=/healthz"`

	// RequireWS controls whether health requires a "fresh WS message" in addition to Redis OK.
	// For offline/dev mode, set RequireWS=false so the service can be healthy without external WS.
	RequireWS bool `json:",default=true"`

	// StaleAfterMillis marks service unhealthy if no WS message newer than this duration.
	StaleAfterMillis int64 `json:",default=5000"`

	// RedisPingTimeoutMillis controls how long the health check waits for Redis PING.
	RedisPingTimeoutMillis int64 `json:",default=500"`
}

// SparklineConfig controls the mini chart (sparkline) data collection.
// Sparklines are simplified price trend charts showing recent price history.
// Data is stored in Redis ZSET with score=timestamp for efficient time-range queries.
type SparklineConfig struct {
	// Enabled controls whether sparkline data collection is active.
	Enabled bool `json:",default=true"`

	// SampleIntervalSeconds is how often to sample prices for sparkline.
	// Default: 300 (5 minutes) → 288 points per 24 hours.
	SampleIntervalSeconds int64 `json:",default=300"`

	// RetentionHours is how long to keep sparkline data.
	// Data older than this is automatically cleaned up via ZREMRANGEBYSCORE.
	// Default: 24 (24 hours). Set to 168 for 7 days.
	RetentionHours int64 `json:",default=24"`
}

// FiatConfig controls periodic fetching of USDT->fiat exchange rates (mid price).
// Data is stored in Redis hash (similar to Binance mini ticker) under RedisHashKey:
//
//	HGET fiat:tickers USDTCNY -> {"buy":"7.20","sell":"7.22","mid":"7.21","ts":1734567890123,"provider":"binance_c2c","currency_symbol":"¥"}
//
// Note: This service is designed for USDT -> {fiat} pairs only (per requirements).
type FiatConfig struct {
	Enabled bool `json:",default=false"`

	// Pairs lists USDT->fiat pairs to fetch periodically.
	// Example:
	//   - Asset: USDT
	//     FiatCurrency: CNY
	//     Enabled: true
	Pairs []FiatPairConfig `json:"Pairs,optional"`

	// UpdateIntervalSeconds controls how often to refresh rates.
	UpdateIntervalSeconds int64 `json:",default=30"`

	// RequestTimeoutMillis is the HTTP request timeout.
	RequestTimeoutMillis int64 `json:",default=10000"`

	// MaxRetries is the max retry count per request (0 disables retries).
	MaxRetries int `json:",default=3"`

	// BaseURL is the API base (default: Binance C2C public API).
	BaseURL string `json:",default=https://c2c.binance.com"`

	// Endpoint is the API path.
	Endpoint string `json:",default=/bapi/c2c/v2/public/c2c/adv/quoted-price"`

	// RedisHashKey stores fiat tickers as hash fields (symbol-like, e.g. USDTCNY).
	RedisHashKey string `json:",default=fiat:tickers"`

	// TTLSeconds sets an expire time on the fiat hash key; 0 disables TTL.
	TTLSeconds int64 `json:",default=600"` // 10 minutes
}

type FiatPairConfig struct {
	Asset        string `json:"Asset"`
	FiatCurrency string `json:"FiatCurrency"`
	Enabled      bool   `json:",default=true"`
}

func (c Config) WSURL() (string, error) {
	base := strings.TrimSpace(c.Binance.BaseURL)
	if base == "" {
		return "", fmt.Errorf("Binance.BaseURL is required")
	}
	u, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("invalid Binance.BaseURL: %w", err)
	}
	if u.Scheme != "wss" {
		return "", fmt.Errorf("Binance.BaseURL must use wss scheme, got %q", u.Scheme)
	}

	endpoint := strings.TrimSpace(c.Binance.Endpoint)
	if endpoint == "" {
		return "", fmt.Errorf("Binance.Endpoint is required")
	}
	if !strings.HasPrefix(endpoint, "/") {
		return "", fmt.Errorf("Binance.Endpoint must start with '/', got %q", endpoint)
	}

	u.Path = strings.TrimRight(u.Path, "/") + endpoint
	return u.String(), nil
}

func (c Config) RedisTTL() time.Duration {
	if c.Redis.TTLSeconds <= 0 {
		return 0
	}
	return time.Duration(c.Redis.TTLSeconds) * time.Second
}

func (c Config) SparklineSampleInterval() time.Duration {
	if c.Sparkline.SampleIntervalSeconds <= 0 {
		return 5 * time.Minute // default
	}
	return time.Duration(c.Sparkline.SampleIntervalSeconds) * time.Second
}

func (c Config) FiatUpdateInterval() time.Duration {
	if c.Fiat.UpdateIntervalSeconds <= 0 {
		return 30 * time.Second
	}
	return time.Duration(c.Fiat.UpdateIntervalSeconds) * time.Second
}

func (c Config) FiatRequestTimeout() time.Duration {
	if c.Fiat.RequestTimeoutMillis <= 0 {
		return 10 * time.Second
	}
	return time.Duration(c.Fiat.RequestTimeoutMillis) * time.Millisecond
}

func (c Config) FiatRedisTTL() time.Duration {
	if c.Fiat.TTLSeconds <= 0 {
		return 0
	}
	return time.Duration(c.Fiat.TTLSeconds) * time.Second
}

// SparklineRetentionMs returns how long to keep sparkline data in milliseconds.
func (c Config) SparklineRetentionMs() int64 {
	hours := c.Sparkline.RetentionHours
	if hours <= 0 {
		hours = 24 // default: 24 hours
	}
	return hours * 60 * 60 * 1000 // convert hours to milliseconds
}

// SparklineRetention returns how long to keep sparkline data as duration.
func (c Config) SparklineRetention() time.Duration {
	hours := c.Sparkline.RetentionHours
	if hours <= 0 {
		hours = 24 // default: 24 hours
	}
	return time.Duration(hours) * time.Hour
}

func (c Config) Validate() error {
	if len(c.CacheRedis) == 0 || strings.TrimSpace(c.CacheRedis[0].Host) == "" {
		return fmt.Errorf("CacheRedis[0].Host is required")
	}
	if strings.TrimSpace(c.Redis.TickerHashKey) == "" {
		return fmt.Errorf("Redis.TickerHashKey is required")
	}
	if c.Redis.TTLSeconds < 0 {
		return fmt.Errorf("Redis.TTLSeconds must be >= 0")
	}
	if c.Redis.DialTimeoutMillis <= 0 || c.Redis.ReadTimeoutMillis <= 0 || c.Redis.WriteTimeoutMillis <= 0 {
		return fmt.Errorf("Redis timeouts must be > 0")
	}
	if c.Redis.PoolSize <= 0 || c.Redis.MinIdleConns < 0 {
		return fmt.Errorf("Redis pool sizes must be valid (PoolSize > 0, MinIdleConns >= 0)")
	}

	if !c.Enabled {
		if c.Sparkline.Enabled {
			return fmt.Errorf("Sparkline.Enabled must be false when Enabled=false")
		}
		if c.Fiat.Enabled {
			return fmt.Errorf("Fiat.Enabled must be false when Enabled=false")
		}
	} else {
		if _, err := c.WSURL(); err != nil {
			return err
		}
		if c.Binance.HandshakeTimeoutMillis <= 0 || c.Binance.ReadTimeoutMillis <= 0 || c.Binance.WriteTimeoutMillis <= 0 {
			return fmt.Errorf("Binance timeouts must be > 0")
		}
		if c.Binance.MaxMessageBytes <= 0 {
			return fmt.Errorf("Binance.MaxMessageBytes must be > 0")
		}
		if c.Binance.MaxConnAgeSeconds <= 0 {
			return fmt.Errorf("Binance.MaxConnAgeSeconds must be > 0")
		}
	}

	if c.Reconnect.InitialBackoffMillis <= 0 {
		return fmt.Errorf("Reconnect.InitialBackoffMillis must be > 0")
	}
	if c.Reconnect.MaxBackoffMillis < c.Reconnect.InitialBackoffMillis {
		return fmt.Errorf("Reconnect.MaxBackoffMillis must be >= Reconnect.InitialBackoffMillis")
	}
	if c.Reconnect.Multiplier < 1 {
		return fmt.Errorf("Reconnect.Multiplier must be >= 1")
	}
	if c.Reconnect.Jitter < 0 || c.Reconnect.Jitter > 1 {
		return fmt.Errorf("Reconnect.Jitter must be between 0 and 1")
	}

	if c.Health.Enabled {
		if strings.TrimSpace(c.Health.ListenOn) == "" {
			return fmt.Errorf("Health.ListenOn is required when Health.Enabled=true")
		}
		if strings.TrimSpace(c.Health.Path) == "" || !strings.HasPrefix(c.Health.Path, "/") {
			return fmt.Errorf("Health.Path must start with '/', got %q", c.Health.Path)
		}
		if c.Health.RequireWS && c.Health.StaleAfterMillis <= 0 {
			return fmt.Errorf("Health.StaleAfterMillis must be > 0 when Health.RequireWS=true")
		}
		if c.Health.RedisPingTimeoutMillis <= 0 {
			return fmt.Errorf("Health.RedisPingTimeoutMillis must be > 0")
		}
	}

	if c.Sparkline.Enabled {
		if strings.TrimSpace(c.Redis.SparklineKeyPrefix) == "" {
			return fmt.Errorf("Redis.SparklineKeyPrefix is required when Sparkline.Enabled=true")
		}
		if c.Sparkline.SampleIntervalSeconds <= 0 {
			return fmt.Errorf("Sparkline.SampleIntervalSeconds must be > 0")
		}
		if c.Sparkline.RetentionHours <= 0 {
			return fmt.Errorf("Sparkline.RetentionHours must be > 0")
		}
	}

	if c.Fiat.Enabled {
		if strings.TrimSpace(c.Fiat.RedisHashKey) == "" {
			return fmt.Errorf("Fiat.RedisHashKey is required when Fiat.Enabled=true")
		}
		if c.Fiat.TTLSeconds < 0 {
			return fmt.Errorf("Fiat.TTLSeconds must be >= 0")
		}
		if c.Fiat.UpdateIntervalSeconds <= 0 {
			return fmt.Errorf("Fiat.UpdateIntervalSeconds must be > 0")
		}
		if c.Fiat.RequestTimeoutMillis <= 0 {
			return fmt.Errorf("Fiat.RequestTimeoutMillis must be > 0")
		}
		if c.Fiat.MaxRetries < 0 {
			return fmt.Errorf("Fiat.MaxRetries must be >= 0")
		}
		base := strings.TrimSpace(c.Fiat.BaseURL)
		if base == "" {
			return fmt.Errorf("Fiat.BaseURL is required when Fiat.Enabled=true")
		}
		u, err := url.Parse(base)
		if err != nil {
			return fmt.Errorf("invalid Fiat.BaseURL: %w", err)
		}
		if u.Scheme != "https" && u.Scheme != "http" {
			return fmt.Errorf("Fiat.BaseURL must use http/https scheme, got %q", u.Scheme)
		}
		endpoint := strings.TrimSpace(c.Fiat.Endpoint)
		if endpoint == "" {
			return fmt.Errorf("Fiat.Endpoint is required when Fiat.Enabled=true")
		}
		if !strings.HasPrefix(endpoint, "/") {
			return fmt.Errorf("Fiat.Endpoint must start with '/', got %q", endpoint)
		}

		for i, p := range c.Fiat.Pairs {
			if !p.Enabled {
				continue
			}
			asset := strings.ToUpper(strings.TrimSpace(p.Asset))
			if asset != "USDT" {
				return fmt.Errorf("Fiat.Pairs[%d].Asset must be USDT, got %q", i, asset)
			}
			fiat := strings.ToUpper(strings.TrimSpace(p.FiatCurrency))
			if fiat == "" {
				return fmt.Errorf("Fiat.Pairs[%d].FiatCurrency is required", i)
			}
		}
	}

	return nil
}
