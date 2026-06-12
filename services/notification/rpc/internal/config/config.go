package config

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf

	// 雪花ID节点编号（0-1023）
	NodeID int64 `json:",optional"`

	// MySQL配置
	MySQL struct {
		Host            string `json:",default=localhost"`
		Port            int    `json:",default=3306"`
		Username        string `json:",default=crypto"`
		Password        string `json:",optional"`
		Database        string `json:",default=crypto_wallet"`
		MaxIdleConns    int    `json:",default=10"`
		MaxOpenConns    int    `json:",default=100"`
		ConnMaxLifetime int    `json:",default=3600"`
		LogLevel        int    `json:",default=4"`
		SlowThreshold   int    `json:",default=200"`
	}

	// Redis缓存配置
	CacheRedis cache.CacheConf

	// 极光推送配置
	JPush struct {
		AppKey         string `json:",optional"`
		MasterSecret   string `json:",optional"`
		ApnsProduction bool   `json:",default=false"`
		Timeout        int    `json:",default=5000"`
		MaxRetries     int    `json:",default=3"`
		RetryInterval  int    `json:",default=1000"`
	}

	// Kafka配置
	Kafka struct {
		Brokers []string `json:",optional"`
		GroupID string   `json:",default=notification-consumer"`
		Topics  struct {
			Transaction string `json:",default=crypto.transaction.events"`
			Security    string `json:",default=crypto.security.events"`
			System      string `json:",default=crypto.system.events"`
			PriceAlert  string `json:",default=crypto.price.alert"`
		}
		EnableConsumer      bool `json:",default=true"`
		ConsumerConcurrency int  `json:",default=5"`
	} `json:"Kafka,optional"`

	// 注意：Telemetry 和 Prometheus 配置已经内置在 zrpc.RpcServerConf 中，无需重复定义
}
