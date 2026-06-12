package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
)

// RedisCacheManager Redis缓存管理器
type RedisCacheManager struct {
	redisClient *redis.Client
	prefix      string
}

// NewRedisCacheManager 创建Redis缓存管理器
func NewRedisCacheManager(redisClient *redis.Client, prefix string) *RedisCacheManager {
	if prefix == "" {
		prefix = "chainsync"
	}
	return &RedisCacheManager{
		redisClient: redisClient,
		prefix:      prefix,
	}
}

// ChainBlockCache 链区块缓存结构
type ChainBlockCache struct {
	ChainType   pb.BlockChainType `json:"chain_type"`
	BlockNumber uint64            `json:"block_number"`
	LastUpdated time.Time         `json:"last_updated"`
	TTL         time.Duration     `json:"ttl"` // 缓存有效期
}

// GetLatestBlockFromRedis 从Redis获取链的最新区块高度
func (rcm *RedisCacheManager) GetLatestBlockFromRedis(chainType pb.BlockChainType) (uint64, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	key := rcm.getChainBlockKey(chainType)

	result, err := rcm.redisClient.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			// Redis中没有缓存
			return 0, false, nil
		}
		logx.Errorf("Failed to get latest block from Redis for chain %v: %v", chainType, err)
		return 0, false, err
	}

	// 尝试解析为整数
	blockNumber, err := strconv.ParseUint(result, 10, 64)
	if err != nil {
		// 如果不是整数，尝试解析为JSON结构（向后兼容）
		var cache ChainBlockCache
		if jsonErr := json.Unmarshal([]byte(result), &cache); jsonErr == nil {
			return cache.BlockNumber, true, nil
		}
		logx.Errorf("Failed to parse block number from Redis for chain %v: %v", chainType, err)
		return 0, false, err
	}

	return blockNumber, true, nil
}

// SetLatestBlockToRedis 设置链的最新区块高度到Redis
func (rcm *RedisCacheManager) SetLatestBlockToRedis(chainType pb.BlockChainType, blockNumber uint64, ttl time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	key := rcm.getChainBlockKey(chainType)

	// 直接存储区块号，不使用JSON结构
	err := rcm.redisClient.Set(ctx, key, blockNumber, ttl).Err()
	if err != nil {
		logx.Errorf("Failed to set latest block to Redis for chain %v: %v", chainType, err)
		return err
	}

	logx.Debugf("✓ Set latest block to Redis for chain %v: %d (TTL: %v)", chainType, blockNumber, ttl)
	return nil
}

// BatchSetLatestBlocks 批量设置多个链的最新区块高度（带延迟刷新机制）
func (rcm *RedisCacheManager) BatchSetLatestBlocks(blocks map[pb.BlockChainType]uint64, ttl time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pipe := rcm.redisClient.Pipeline()

	for chainType, blockNumber := range blocks {
		key := rcm.getChainBlockKey(chainType)
		pipe.Set(ctx, key, blockNumber, ttl)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		logx.Errorf("Failed to batch set latest blocks to Redis: %v", err)
		return err
	}

	logx.Infof("✓ Batch set latest blocks to Redis for %d chains", len(blocks))
	return nil
}

// getChainBlockKey 获取链区块缓存key
func (rcm *RedisCacheManager) getChainBlockKey(chainType pb.BlockChainType) string {
	return fmt.Sprintf("%s:block:%v", rcm.prefix, chainType)
}

// Get 通用获取方法
func (rcm *RedisCacheManager) Get(key string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	fullKey := fmt.Sprintf("%s:%s", rcm.prefix, key)
	result, err := rcm.redisClient.Get(ctx, fullKey).Result()
	if err != nil {
		if err == redis.Nil {
			return "", nil // Redis中没有缓存
		}
		return "", err
	}

	return result, nil
}

// SetWithTTL 通用设置方法（带过期时间）
func (rcm *RedisCacheManager) SetWithTTL(key string, value string, ttl int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	fullKey := fmt.Sprintf("%s:%s", rcm.prefix, key)
	duration := time.Duration(ttl) * time.Second

	err := rcm.redisClient.Set(ctx, fullKey, value, duration).Err()
	if err != nil {
		logx.Errorf("Failed to set cache for key %s: %v", key, err)
		return err
	}

	logx.Debugf("✓ Set cache for key %s (TTL: %ds)", key, ttl)
	return nil
}

// Set sets a key without expiration (persistent cursor/state).
func (rcm *RedisCacheManager) Set(key string, value string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	fullKey := fmt.Sprintf("%s:%s", rcm.prefix, key)
	if err := rcm.redisClient.Set(ctx, fullKey, value, 0).Err(); err != nil {
		logx.Errorf("Failed to set cache for key %s: %v", key, err)
		return err
	}
	return nil
}
