package config

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/zrpc"
)

type ChainConfig struct {
	ChainType    string `json:",default=ETH"`  // ETH, BSC, TRON
	ChainID      int64  `json:",default=1"`    // Chain ID
	RPCEndpoint  string `json:",optional"`     // RPC endpoint for ETH/BSC
	APIKey       string `json:",optional"`     // API key for RPC node authentication
	TronEndpoint string `json:",optional"`     // gRPC endpoint for TRON
	TronAPIKey   string `json:",optional"`     // API key for TRON node authentication
	Enabled      bool   `json:",default=true"` // Whether this chain is enabled
}

type SignerConfig struct {
	Host string `json:",default=localhost"`
	Port int    `json:",default=8081"`
}

type Config struct {
	zrpc.RpcServerConf
	// NodeID 用于 Snowflake / 服务实例标识（建议为每个服务分配唯一值）
	NodeID    int64              `json:",optional"`
	Chains    []ChainConfig      `json:",optional"`
	Signer    SignerConfig       `json:",optional"` // 保持向后兼容
	SignerRpc zrpc.RpcClientConf `json:",optional"` // 新的etcd配置

	// Redis 缓存配置（用于从 market 服务获取价格）
	CacheRedis cache.CacheConf `json:",optional"`

	// Market 价格服务的 Redis 配置
	MarketPrice struct {
		TickerHashKey string `json:",default=binance:tickers"` // Market 服务存储价格的 Redis hash key
	} `json:"MarketPrice,optional"`
}
