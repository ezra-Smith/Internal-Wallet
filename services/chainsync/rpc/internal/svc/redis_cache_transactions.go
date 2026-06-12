package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
)

// TransactionCache 交易缓存结构（用于去重）
type TransactionCache struct {
	TxHash      string            `json:"tx_hash"`
	ChainType   pb.BlockChainType `json:"chain_type"`
	BlockNumber uint64            `json:"block_number"`
	ProcessedAt time.Time         `json:"processed_at"`
	IsSent      bool              `json:"is_sent"` // 是否已发送到消息队列
}

// IsTransactionProcessed 检查交易是否已处理（去重）
func (rcm *RedisCacheManager) IsTransactionProcessed(txHash string, chainType pb.BlockChainType) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	key := rcm.getTransactionKey(txHash, chainType)

	exists, err := rcm.redisClient.Exists(ctx, key).Result()
	if err != nil {
		logx.Errorf("Failed to check transaction exists in Redis for %s: %v", txHash, err)
		return false, err
	}

	return exists > 0, nil
}

// MarkTransactionProcessed 标记交易为已处理
func (rcm *RedisCacheManager) MarkTransactionProcessed(tx *pb.Transaction, ttl time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	key := rcm.getTransactionKey(tx.TxHash, tx.Chain)

	cache := TransactionCache{
		TxHash:      tx.TxHash,
		ChainType:   tx.Chain,
		BlockNumber: tx.BlockNumber,
		ProcessedAt: time.Now(),
		IsSent:      false,
	}

	data, err := json.Marshal(cache)
	if err != nil {
		logx.Errorf("Failed to marshal transaction cache for %s: %v", tx.TxHash, err)
		return err
	}

	if err := rcm.redisClient.Set(ctx, key, data, ttl).Err(); err != nil {
		logx.Errorf("Failed to mark transaction as processed in Redis for %s: %v", tx.TxHash, err)
		return err
	}

	logx.Debugf("✓ Marked transaction as processed: %s", tx.TxHash)
	return nil
}

// MarkTransactionSent 标记交易已发送到消息队列
func (rcm *RedisCacheManager) MarkTransactionSent(txHash string, chainType pb.BlockChainType) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	key := rcm.getTransactionKey(txHash, chainType)

	data, err := rcm.redisClient.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			cache := TransactionCache{
				TxHash:      txHash,
				ChainType:   chainType,
				BlockNumber: 0,
				ProcessedAt: time.Now(),
				IsSent:      true,
			}
			data, jsonErr := json.Marshal(cache)
			if jsonErr != nil {
				return jsonErr
			}
			return rcm.redisClient.Set(ctx, key, data, 24*time.Hour).Err()
		}
		return err
	}

	var cache TransactionCache
	if jsonErr := json.Unmarshal([]byte(data), &cache); jsonErr != nil {
		return jsonErr
	}

	cache.IsSent = true
	cache.ProcessedAt = time.Now()

	updatedData, jsonErr := json.Marshal(cache)
	if jsonErr != nil {
		return jsonErr
	}

	return rcm.redisClient.Set(ctx, key, updatedData, 24*time.Hour).Err()
}

// GetUnsentTransactions 获取未发送的交易列表（用于重发）
func (rcm *RedisCacheManager) GetUnsentTransactions(chainType pb.BlockChainType, limit int) ([]*TransactionCache, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pattern := fmt.Sprintf("%s:tx:%v:*", rcm.prefix, chainType)

	var unsentTxs []*TransactionCache
	var cursor uint64
	for {
		keys, next, err := rcm.redisClient.Scan(ctx, cursor, pattern, 200).Result()
		if err != nil {
			return nil, err
		}
		cursor = next

		for _, key := range keys {
			data, err := rcm.redisClient.Get(ctx, key).Result()
			if err != nil {
				continue
			}

			var cache TransactionCache
			if err := json.Unmarshal([]byte(data), &cache); err != nil {
				continue
			}

			if !cache.IsSent {
				unsentTxs = append(unsentTxs, &cache)
				if limit > 0 && len(unsentTxs) >= limit {
					return unsentTxs, nil
				}
			}
		}

		if cursor == 0 {
			break
		}
	}

	return unsentTxs, nil
}

// CleanupOldTransactions 清理过期的交易缓存
func (rcm *RedisCacheManager) CleanupOldTransactions(maxAge time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pattern := fmt.Sprintf("%s:tx:*", rcm.prefix)

	cutoffTime := time.Now().Add(-maxAge)
	deletedCount := 0

	var cursor uint64
	for {
		keys, next, err := rcm.redisClient.Scan(ctx, cursor, pattern, 500).Result()
		if err != nil {
			return err
		}
		cursor = next

		for _, key := range keys {
			data, err := rcm.redisClient.Get(ctx, key).Result()
			if err != nil {
				continue
			}

			var cache TransactionCache
			if err := json.Unmarshal([]byte(data), &cache); err != nil {
				continue
			}

			if cache.ProcessedAt.Before(cutoffTime) {
				if err := rcm.redisClient.Del(ctx, key).Err(); err == nil {
					deletedCount++
				}
			}
		}

		if cursor == 0 {
			break
		}
	}

	logx.Infof("✓ Cleaned up %d expired transaction cache entries", deletedCount)
	return nil
}

// getTransactionKey 获取交易缓存key
func (rcm *RedisCacheManager) getTransactionKey(txHash string, chainType pb.BlockChainType) string {
	return fmt.Sprintf("%s:tx:%v:%s", rcm.prefix, chainType, txHash)
}
