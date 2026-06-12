package svc

import (
	"context"
	"time"

	"internalwallet/services/market/rpc/internal/config"
	"internalwallet/services/market/rpc/internal/status"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
)

type ServiceContext struct {
	Config      config.Config
	RedisClient *redis.Client
	Status      *status.ServiceStatus
}

func NewServiceContext(c config.Config) *ServiceContext {
	redisAddr := c.CacheRedis[0].Host
	redisClient := redis.NewClient(&redis.Options{
		Addr:         redisAddr,
		Password:     c.CacheRedis[0].Pass,
		DB:           c.Redis.DB,
		PoolSize:     c.Redis.PoolSize,
		MinIdleConns: c.Redis.MinIdleConns,
		MaxRetries:   c.Redis.MaxRetries,
		DialTimeout:  time.Duration(c.Redis.DialTimeoutMillis) * time.Millisecond,
		ReadTimeout:  time.Duration(c.Redis.ReadTimeoutMillis) * time.Millisecond,
		WriteTimeout: time.Duration(c.Redis.WriteTimeoutMillis) * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		logx.Errorf("Failed to connect to Redis (%s): %v", redisAddr, err)
	} else {
		logx.Infof("✓ Connected to Redis (%s, DB=%d)", redisAddr, c.Redis.DB)
	}

	return &ServiceContext{
		Config:      c,
		RedisClient: redisClient,
		Status:      status.New(),
	}
}
