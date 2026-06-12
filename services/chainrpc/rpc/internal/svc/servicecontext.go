package svc

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/services/chainrpc/rpc/internal/config"
)

type ServiceContext struct {
	Config       config.Config
	ChainMgr     *ChainClientManager
	RedisClient  *redis.Client
	NonceManager *NonceManager
}

func NewServiceContext(c config.Config) (*ServiceContext, error) {
	chainMgr, err := NewChainClientManager(c)
	if err != nil {
		return nil, err
	}

	// 初始化 Redis 客户端（用于从 market 服务获取价格）
	var redisClient *redis.Client
	if len(c.CacheRedis) > 0 && c.CacheRedis[0].Host != "" {
		redisClient = redis.NewClient(&redis.Options{
			Addr:         c.CacheRedis[0].Host,
			Password:     c.CacheRedis[0].Pass,
			DB:           0,
			PoolSize:     50,
			MinIdleConns: 10,
			MaxRetries:   3,
			DialTimeout:  5 * time.Second,
			ReadTimeout:  3 * time.Second,
			WriteTimeout: 3 * time.Second,
		})

		// 测试连接
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := redisClient.Ping(ctx).Err(); err != nil {
			if c.CacheRedis[0].Pass == "" {
				logx.Errorf("Failed to connect to Redis (%s): %v", c.CacheRedis[0].Host, err)
				logx.Errorf("⚠️  Redis password is empty. Please set REDIS_PASSWORD in .env file or environment variables")
				logx.Errorf("⚠️  Example: REDIS_PASSWORD=redis123456")
			} else {
				logx.Errorf("Failed to connect to Redis (%s): %v", c.CacheRedis[0].Host, err)
			}
			logx.Infof("⚠️  Service will continue without Redis. Gas fee USD prices will use default values.")
			// 不返回错误，允许服务在没有 Redis 的情况下运行（价格会使用默认值）
			redisClient = nil
		} else {
			logx.Infof("✓ Connected to Redis (%s) for market price data", c.CacheRedis[0].Host)
		}
	}

	return &ServiceContext{
		Config:       c,
		ChainMgr:     chainMgr,
		RedisClient:  redisClient,
		NonceManager: NewNonceManager(2 * time.Minute),
	}, nil
}
