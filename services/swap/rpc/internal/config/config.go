package config

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/zrpc"

	commonDB "internalwallet/common/db"
)

type Config struct {
	zrpc.RpcServerConf

	// 雪花ID配置（必须配置，范围0-1023，每个服务唯一）
	NodeID int64 `json:",optional"`

	// GORM MySQL 配置（使用通用配置）
	MySQL commonDB.MySQLConfig `json:",optional"`

	// Redis 缓存配置
	CacheRedis cache.CacheConf

	// Signer RPC 配置（用于交易签名）
	SignerRpc zrpc.RpcClientConf `json:",optional"`

	Swap SwapConfig

	// 数据库自动迁移配置（仅开发/测试环境建议启用）
	AutoMigrate bool `json:",optional,default=false"`
}

type SwapConfig struct {
	Auth     AuthConfig
	Provider ProviderConfig
	Chains   []ChainConfig `json:",optional"`
	Cache    CacheConfig
}

type AuthConfig struct {
	// 用于对外 API key 进行哈希的 pepper（必须通过环境变量覆盖，不要提交到 git）
	ApiKeyPepper string `json:",optional"`

	// 管理端 Token 的 sha256(hex)（用于调用 Create/Revoke/ListSwapApiKeys）
	// 推荐通过环境变量注入：SWAP_ADMIN_TOKEN 或 SWAP_ADMIN_TOKEN_SHA256
	AdminTokenHash string `json:",optional"`

	// 限流档位配置（按 swap_svc_api_keys.rate_limit_tier）
	// 例如：
	//   default: { requests_per_minute: 60 }
	//   pro:     { requests_per_minute: 600 }
	RateLimitTiers map[string]RateLimitTier `json:",optional"`

	// 未匹配到 tier 时使用的默认档位
	DefaultTier string `json:",optional,default=default"`
}

type RateLimitTier struct {
	RequestsPerMinute int `json:",optional,default=60"`
}

type ProviderConfig struct {
	// Provider type (extensible by factory), e.g. "1inch" / "okx".
	Type    string        `json:",optional,default=1inch"`
	OneInch OneInchConfig `json:",optional"`
	Okx     OkxConfig     `json:",optional"`
}

type OneInchConfig struct {
	BaseURL string `json:",optional,default=https://api.1inch.dev"`
	// API versioned base path, e.g. /swap/v6.0
	SwapBasePath string `json:",optional,default=/swap/v6.0"`

	// 1inch API Key（必须通过环境变量覆盖，不要提交到 git）
	ApiKey string `json:",optional"`

	TimeoutMillis        int `json:",optional,default=8000"`
	MaxRetries           int `json:",optional,default=2"`
	RetryBaseDelayMillis int `json:",optional,default=200"`

	// 对 1inch 上游的客户端限速（RPS）；0 表示不限制
	RateLimitRPS int `json:",optional,default=15"`
}

// OkxConfig is used by the OKX DEX Aggregator provider.
//
// Docs:
// - https://web3.okx.com/build/dev-docs-v5/dex-api/dex-api-access-and-usage
// - https://web3.okx.com/build/dev-docs-v5/dex-api/dex-swap
type OkxConfig struct {
	BaseURL string `json:",optional,default=https://web3.okx.com"`
	// API base path, e.g. /api/v5/dex/aggregator or /api/v6/dex/aggregator.
	BasePath string `json:",optional,default=/api/v6/dex/aggregator"`

	// OKX API credentials (must be provided via env vars; never commit secrets).
	ApiKey     string `json:",optional"`
	SecretKey  string `json:",optional"`
	Passphrase string `json:",optional"`

	TimeoutMillis        int `json:",optional,default=8000"`
	MaxRetries           int `json:",optional,default=2"`
	RetryBaseDelayMillis int `json:",optional,default=200"`

	// Client-side upstream rate limit (RPS); 0 means unlimited.
	RateLimitRPS int `json:",optional,default=15"`
}

type ChainConfig struct {
	ChainID               int64    `json:",optional"`
	Name                  string   `json:",optional"`
	Enabled               bool     `json:",optional,default=true"`
	RPCEndpoints          []string `json:",optional"`
	RequiredConfirmations uint64   `json:",optional,default=12"`
}

type CacheConfig struct {
	// tokens 列表缓存（秒）
	TokensTTLSeconds int `json:",optional,default=43200"` // 12h
}
