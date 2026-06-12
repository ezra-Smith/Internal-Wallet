package svc

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
)

// SmartCacheConfig 智能缓存配置
type SmartCacheConfig struct {
	AddressCache struct {
		ShortTTL  time.Duration // 短期缓存时间（活跃地址）
		LongTTL   time.Duration // 长期缓存时间（不活跃地址）
		Threshold int           // 活跃阈值（访问次数）
	}

	BlockCache struct {
		DefaultTTL time.Duration // 默认缓存时间
		FreshTTL   time.Duration // 新鲜数据缓存时间
	}

	TransactionCache struct {
		DefaultTTL   time.Duration // 默认缓存时间
		ConfirmedTTL time.Duration // 已确认交易缓存时间
	}
}

// GetSmartCacheConfig 获取智能缓存配置
func (rcm *RedisCacheManager) GetSmartCacheConfig() SmartCacheConfig {
	return SmartCacheConfig{
		AddressCache: struct {
			ShortTTL  time.Duration
			LongTTL   time.Duration
			Threshold int
		}{
			ShortTTL:  1 * time.Hour,
			LongTTL:   24 * time.Hour,
			Threshold: 5,
		},
		BlockCache: struct {
			DefaultTTL time.Duration
			FreshTTL   time.Duration
		}{
			DefaultTTL: 5 * time.Minute,
			FreshTTL:   1 * time.Minute,
		},
		TransactionCache: struct {
			DefaultTTL   time.Duration
			ConfirmedTTL time.Duration
		}{
			DefaultTTL:   24 * time.Hour,
			ConfirmedTTL: 7 * 24 * time.Hour,
		},
	}
}

// SmartSetAddressLastProcessedBlock 智能设置地址最后处理区块
func (rcm *RedisCacheManager) SmartSetAddressLastProcessedBlock(address, chainType string, blockNumber uint64) error {
	config := rcm.GetSmartCacheConfig()

	accessKey := fmt.Sprintf("addr:access:%s:%s", chainType, address)
	accessCount, err := rcm.incrementAddressAccess(accessKey)
	if err != nil {
		logx.Errorf("Failed to track address access for %s: %v", address, err)
	}

	var ttl time.Duration
	if accessCount >= config.AddressCache.Threshold {
		ttl = config.AddressCache.ShortTTL
	} else {
		ttl = config.AddressCache.LongTTL
	}

	addressCacheKey := fmt.Sprintf("addr:last_block:%s:%s", chainType, address)
	return rcm.SetWithTTL(addressCacheKey, strconv.FormatUint(blockNumber, 10), int(ttl.Seconds()))
}

// incrementAddressAccess 增加地址访问计数
func (rcm *RedisCacheManager) incrementAddressAccess(key string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	fullKey := fmt.Sprintf("%s:%s", rcm.prefix, key)

	result, err := rcm.redisClient.Incr(ctx, fullKey).Result()
	if err != nil {
		return 0, err
	}

	if result == 1 {
		rcm.redisClient.Expire(ctx, fullKey, 24*time.Hour)
	}

	return int(result), nil
}

// GetAddressActivityLevel 获取地址活跃度
func (rcm *RedisCacheManager) GetAddressActivityLevel(address, chainType string) (int, error) {
	accessKey := fmt.Sprintf("addr:access:%s:%s", chainType, address)

	value, err := rcm.Get(accessKey)
	if err != nil || value == "" {
		return 0, err
	}

	return strconv.Atoi(value)
}

// SmartSetLatestBlock 智能设置最新区块缓存
func (rcm *RedisCacheManager) SmartSetLatestBlock(chainType pb.BlockChainType, blockNumber uint64, isFresh bool) error {
	config := rcm.GetSmartCacheConfig()

	var ttl time.Duration
	if isFresh {
		ttl = config.BlockCache.FreshTTL
	} else {
		ttl = config.BlockCache.DefaultTTL
	}

	return rcm.SetLatestBlockToRedis(chainType, blockNumber, ttl)
}

// SmartMarkTransactionProcessed 智能标记交易已处理
func (rcm *RedisCacheManager) SmartMarkTransactionProcessed(tx *pb.Transaction, isConfirmed bool) error {
	config := rcm.GetSmartCacheConfig()

	var ttl time.Duration
	if isConfirmed {
		ttl = config.TransactionCache.ConfirmedTTL
	} else {
		ttl = config.TransactionCache.DefaultTTL
	}

	return rcm.MarkTransactionProcessed(tx, ttl)
}

// BatchGetAddressLastProcessedBlocks 批量获取地址最后处理区块
func (rcm *RedisCacheManager) BatchGetAddressLastProcessedBlocks(addresses []string, chainType string) (map[string]uint64, error) {
	if len(addresses) == 0 {
		return make(map[string]uint64), nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	keys := make([]string, 0, len(addresses))
	keyToAddress := make(map[string]string)

	for _, addr := range addresses {
		key := fmt.Sprintf("%s:addr:last_block:%s:%s", rcm.prefix, chainType, addr)
		keys = append(keys, key)
		keyToAddress[key] = addr
	}

	results, err := rcm.redisClient.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	blocks := make(map[string]uint64)
	for i, result := range results {
		if result == nil {
			continue
		}
		if blockStr, ok := result.(string); ok && blockStr != "" {
			if blockNum, parseErr := strconv.ParseUint(blockStr, 10, 64); parseErr == nil {
				originalKey := keys[i]
				if address, exists := keyToAddress[originalKey]; exists {
					blocks[address] = blockNum
				}
			}
		}
	}

	return blocks, nil
}

// BatchSetAddressLastProcessedBlocks 批量设置地址最后处理区块
func (rcm *RedisCacheManager) BatchSetAddressLastProcessedBlocks(addresses []string, chainType string, blockNumber uint64) error {
	if len(addresses) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	config := rcm.GetSmartCacheConfig()
	pipe := rcm.redisClient.Pipeline()

	for _, addr := range addresses {
		accessKey := fmt.Sprintf("addr:access:%s:%s", chainType, addr)
		accessCount, _ := rcm.incrementAddressAccess(accessKey)

		var ttl time.Duration
		if accessCount >= config.AddressCache.Threshold {
			ttl = config.AddressCache.ShortTTL
		} else {
			ttl = config.AddressCache.LongTTL
		}

		cacheKey := fmt.Sprintf("%s:addr:last_block:%s:%s", rcm.prefix, chainType, addr)
		pipe.Set(ctx, cacheKey, strconv.FormatUint(blockNumber, 10), ttl)
	}

	_, err := pipe.Exec(ctx)
	return err
}

// GetCacheStatistics 获取缓存统计信息
func (rcm *RedisCacheManager) GetCacheStatistics(_ string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pattern := fmt.Sprintf("%s:*", rcm.prefix)
	stats := make(map[string]interface{})

	blockKeys := 0
	txKeys := 0
	addrKeys := 0
	accessKeys := 0

	totalKeys := 0
	var cursor uint64
	for {
		keys, next, err := rcm.redisClient.Scan(ctx, cursor, pattern, 500).Result()
		if err != nil {
			return nil, err
		}
		cursor = next

		totalKeys += len(keys)
		for _, key := range keys {
			if strings.Contains(key, ":block:") {
				blockKeys++
			} else if strings.Contains(key, ":tx:") {
				txKeys++
			} else if strings.Contains(key, ":addr:") {
				if strings.Contains(key, ":access:") {
					accessKeys++
				} else {
					addrKeys++
				}
			}
		}

		if cursor == 0 {
			break
		}
	}

	stats["total_keys"] = totalKeys
	stats["block_caches"] = blockKeys
	stats["transaction_caches"] = txKeys
	stats["address_caches"] = addrKeys
	stats["access_counters"] = accessKeys

	info, err := rcm.redisClient.Info(ctx, "memory").Result()
	if err == nil {
		stats["memory_info"] = info
	}

	return stats, nil
}

// CleanupExpiredCaches 清理过期缓存
func (rcm *RedisCacheManager) CleanupExpiredCaches() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	patterns := []string{
		fmt.Sprintf("%s:addr:access:*", rcm.prefix),
	}

	totalDeleted := 0
	for _, pattern := range patterns {
		var cursor uint64
		for {
			keys, next, err := rcm.redisClient.Scan(ctx, cursor, pattern, 500).Result()
			if err != nil {
				logx.Errorf("Failed to scan keys for pattern %s: %v", pattern, err)
				break
			}
			cursor = next

			if len(keys) > 0 {
				deleted, err := rcm.redisClient.Del(ctx, keys...).Result()
				if err != nil {
					logx.Errorf("Failed to delete keys for pattern %s: %v", pattern, err)
				} else {
					totalDeleted += int(deleted)
				}
			}

			if cursor == 0 {
				break
			}
		}
	}

	if totalDeleted > 0 {
		logx.Infof("🧹 Cleaned up %d expired cache entries", totalDeleted)
	}

	return nil
}
