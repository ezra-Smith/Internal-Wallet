package svc

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
	"internalwallet/services/chainsync/rpc/internal/model"
	"internalwallet/services/chainsync/rpc/internal/provider"
	"internalwallet/services/chainsync/rpc/internal/repository"
)

// ReorgDetector 区块重组检测器
type ReorgDetector struct {
	providerPool      *provider.Pool
	unconfirmedTxRepo repository.UnconfirmedTransactionRepository
	kafkaProducer     KafkaProducerInterface
	kafkaRetryQueue   *KafkaRetryQueue

	// 区块哈希缓存: chain -> blockNumber -> blockHash
	blockHashCache map[pb.BlockChainType]map[uint64]string
	cacheMu        sync.RWMutex

	// 配置
	checkDepth uint64 // 检测深度(检查最近N个区块)

	// 统计
	stats *ReorgStats
	mu    sync.RWMutex
}

// ReorgStats 重组检测统计
type ReorgStats struct {
	TotalChecks       uint64    `json:"total_checks"`
	ReorgsDetected    uint64    `json:"reorgs_detected"`
	TransactionsFixed uint64    `json:"transactions_fixed"`
	LastCheckTime     time.Time `json:"last_check_time"`
	LastReorgTime     time.Time `json:"last_reorg_time"`
}

// ReorgEvent 重组事件
type ReorgEvent struct {
	Chain           pb.BlockChainType `json:"chain"`
	BlockNumber     uint64            `json:"block_number"`
	OldBlockHash    string            `json:"old_block_hash"`
	NewBlockHash    string            `json:"new_block_hash"`
	AffectedTxCount int               `json:"affected_tx_count"`
	DetectedAt      time.Time         `json:"detected_at"`
}

// NewReorgDetector 创建重组检测器
func NewReorgDetector(
	providerPool *provider.Pool,
	unconfirmedTxRepo repository.UnconfirmedTransactionRepository,
	kafkaProducer KafkaProducerInterface,
	kafkaRetryQueue *KafkaRetryQueue,
) *ReorgDetector {
	return &ReorgDetector{
		providerPool:      providerPool,
		unconfirmedTxRepo: unconfirmedTxRepo,
		kafkaProducer:     kafkaProducer,
		kafkaRetryQueue:   kafkaRetryQueue,
		blockHashCache:    make(map[pb.BlockChainType]map[uint64]string),
		checkDepth:        DefaultReorgCheckDepth,
		stats:             &ReorgStats{},
	}
}

// VerifyTransactionBlock 验证交易所在区块是否仍在主链上
// 返回: isValid(区块仍在主链), currentBlockHash(当前区块哈希), error
func (rd *ReorgDetector) VerifyTransactionBlock(ctx context.Context, tx *model.UnconfirmedTransaction) (bool, string, error) {
	if tx == nil {
		return false, "", fmt.Errorf("transaction is nil")
	}

	chainType := rd.parseChainType(tx.Chain)

	// 获取Provider
	provider, err := rd.providerPool.GetProvider(chainType)
	if err != nil {
		return false, "", fmt.Errorf("failed to get provider: %v", err)
	}

	// 获取区块信息
	block, err := provider.GetBlock(context.Background(), tx.BlockNumber)
	if err != nil {
		return false, "", fmt.Errorf("failed to get block %d: %v", tx.BlockNumber, err)
	}

	// 比较区块哈希
	currentBlockHash := block.BlockHash
	isValid := currentBlockHash == tx.BlockHash

	if !isValid {
		logx.Infof("⚠️ Block reorg detected for tx %s: block %d hash changed from %s to %s",
			tx.TxHash, tx.BlockNumber, tx.BlockHash, currentBlockHash)

		rd.mu.Lock()
		rd.stats.ReorgsDetected++
		rd.stats.LastReorgTime = time.Now()
		rd.mu.Unlock()
	}

	rd.mu.Lock()
	rd.stats.TotalChecks++
	rd.stats.LastCheckTime = time.Now()
	rd.mu.Unlock()

	return isValid, currentBlockHash, nil
}

// HandleReorgedTransaction 处理被重组的交易
func (rd *ReorgDetector) HandleReorgedTransaction(ctx context.Context, tx *model.UnconfirmedTransaction, newBlockHash string) error {
	if tx == nil {
		return fmt.Errorf("transaction is nil")
	}

	logx.Infof("🔄 Handling reorged transaction %s", tx.TxHash)

	chainType := rd.parseChainType(tx.Chain)

	// 1. 尝试在新区块中查找交易
	provider, err := rd.providerPool.GetProvider(chainType)
	if err != nil {
		return fmt.Errorf("failed to get provider: %v", err)
	}

	// 查找交易的当前状态
	currentTx, err := provider.GetTransaction(ctx, tx.TxHash)
	if err != nil {
		// 交易可能不存在(被丢弃)
		logx.Infof("⚠️ Transaction %s not found on chain, may have been dropped", tx.TxHash)
		return rd.markTransactionAsOrphaned(ctx, tx)
	}

	if currentTx != nil && currentTx.BlockNumber > 0 {
		// 交易在新区块中找到，更新区块信息
		logx.Infof("✅ Transaction %s found in new block %d", tx.TxHash, currentTx.BlockNumber)
		return rd.updateTransactionBlock(ctx, tx, currentTx.BlockNumber, currentTx.BlockHash)
	}

	// 交易不在任何已确认区块中
	return rd.markTransactionAsOrphaned(ctx, tx)
}

// markTransactionAsOrphaned 标记交易为孤块状态
func (rd *ReorgDetector) markTransactionAsOrphaned(ctx context.Context, tx *model.UnconfirmedTransaction) error {
	if rd.unconfirmedTxRepo == nil {
		return nil
	}

	// 使用独立的超时上下文，避免父上下文超时影响数据库操作
	dbCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 标记为重组状态
	if err := rd.unconfirmedTxRepo.MarkAsFailed(dbCtx, uint64(tx.ID), "block reorganization detected, transaction orphaned"); err != nil {
		logx.Errorf("Failed to mark transaction %s as orphaned: %v", tx.TxHash, err)
		return err
	}

	// 发送重组通知
	rd.sendReorgNotification(ctx, tx, "orphaned")

	rd.mu.Lock()
	rd.stats.TransactionsFixed++
	rd.mu.Unlock()

	logx.Infof("⚠️ Transaction %s marked as orphaned due to block reorg", tx.TxHash)
	return nil
}

// updateTransactionBlock 更新交易的区块信息
func (rd *ReorgDetector) updateTransactionBlock(ctx context.Context, tx *model.UnconfirmedTransaction, newBlockNumber uint64, newBlockHash string) error {
	// 这里需要在repository中添加一个更新区块信息的方法
	// 暂时通过重置状态来处理
	if rd.unconfirmedTxRepo == nil {
		return nil
	}

	// 使用独立的超时上下文，避免父上下文超时影响数据库操作
	dbCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 重置确认数，让交易重新进入确认流程
	if err := rd.unconfirmedTxRepo.UpdateConfirmations(dbCtx, uint64(tx.ID), 0); err != nil {
		logx.Errorf("Failed to reset confirmations for transaction %s: %v", tx.TxHash, err)
		return err
	}

	// 发送重组通知
	rd.sendReorgNotification(ctx, tx, "relocated")

	rd.mu.Lock()
	rd.stats.TransactionsFixed++
	rd.mu.Unlock()

	logx.Infof("✅ Transaction %s block updated: %d → %d", tx.TxHash, tx.BlockNumber, newBlockNumber)
	return nil
}

// sendReorgNotification 发送重组通知到Kafka
func (rd *ReorgDetector) sendReorgNotification(ctx context.Context, tx *model.UnconfirmedTransaction, eventType string) {
	if rd.kafkaProducer == nil {
		return
	}

	notification := map[string]interface{}{
		"event_type":   "block_reorg",
		"reorg_type":   eventType, // "orphaned" or "relocated"
		"tx_hash":      tx.TxHash,
		"chain":        tx.Chain,
		"block_number": tx.BlockNumber,
		"block_hash":   tx.BlockHash,
		"from_address": tx.FromAddress,
		"to_address":   tx.ToAddress,
		"value":        tx.Value,
		"detected_at":  time.Now().Unix(),
	}

	topic := fmt.Sprintf("wallet.reorg.%s", tx.Chain)
	key := tx.MonitoredAddress

	messageID, err := rd.kafkaProducer.SendMessage(topic, key, notification)
	if err != nil {
		logx.Errorf("Failed to send reorg notification for tx %s: %v", tx.TxHash, err)

		// 加入重试队列
		if rd.kafkaRetryQueue != nil {
			if enqueueErr := rd.kafkaRetryQueue.EnqueueFailedMessage(
				ctx,
				topic,
				key,
				notification,
				"block_reorg",
				tx.TxHash,
				tx.Chain,
				"reorg_detector",
				1, // 高优先级
			); enqueueErr != nil {
				logx.Errorf("Failed to enqueue reorg notification for retry: %v", enqueueErr)
			}
		}
		return
	}

	logx.Infof("📤 Sent reorg notification for tx %s: messageID=%s, type=%s", tx.TxHash, messageID, eventType)
}

// CacheBlockHash 缓存区块哈希
func (rd *ReorgDetector) CacheBlockHash(chain pb.BlockChainType, blockNumber uint64, blockHash string) {
	rd.cacheMu.Lock()
	defer rd.cacheMu.Unlock()

	if _, exists := rd.blockHashCache[chain]; !exists {
		rd.blockHashCache[chain] = make(map[uint64]string)
	}

	rd.blockHashCache[chain][blockNumber] = blockHash

	// 清理过旧的缓存(只保留最近1000个区块)
	if len(rd.blockHashCache[chain]) > 1000 {
		minBlock := blockNumber - 1000
		for bn := range rd.blockHashCache[chain] {
			if bn < minBlock {
				delete(rd.blockHashCache[chain], bn)
			}
		}
	}
}

// GetCachedBlockHash 获取缓存的区块哈希
func (rd *ReorgDetector) GetCachedBlockHash(chain pb.BlockChainType, blockNumber uint64) (string, bool) {
	rd.cacheMu.RLock()
	defer rd.cacheMu.RUnlock()

	if chainCache, exists := rd.blockHashCache[chain]; exists {
		if hash, found := chainCache[blockNumber]; found {
			return hash, true
		}
	}
	return "", false
}

// DetectReorgInRange 检测指定区块范围内的重组
func (rd *ReorgDetector) DetectReorgInRange(ctx context.Context, chain pb.BlockChainType, startBlock, endBlock uint64) ([]ReorgEvent, error) {
	provider, err := rd.providerPool.GetProvider(chain)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider: %v", err)
	}

	var reorgEvents []ReorgEvent

	for blockNum := startBlock; blockNum <= endBlock; blockNum++ {
		// 获取当前链上的区块哈希
		block, err := provider.GetBlock(ctx, blockNum)
		if err != nil {
			logx.Errorf("Failed to get block %d for reorg detection: %v", blockNum, err)
			continue
		}

		// 检查是否与缓存的哈希不同
		if cachedHash, exists := rd.GetCachedBlockHash(chain, blockNum); exists {
			if cachedHash != block.BlockHash {
				// 检测到重组
				event := ReorgEvent{
					Chain:        chain,
					BlockNumber:  blockNum,
					OldBlockHash: cachedHash,
					NewBlockHash: block.BlockHash,
					DetectedAt:   time.Now(),
				}
				reorgEvents = append(reorgEvents, event)

				logx.Infof("🔄 Reorg detected at block %d on chain %v: %s → %s",
					blockNum, chain, cachedHash, block.BlockHash)
			}
		}

		// 更新缓存
		rd.CacheBlockHash(chain, blockNum, block.BlockHash)
	}

	return reorgEvents, nil
}

// GetStats 获取统计信息
func (rd *ReorgDetector) GetStats() *ReorgStats {
	rd.mu.RLock()
	defer rd.mu.RUnlock()

	return &ReorgStats{
		TotalChecks:       rd.stats.TotalChecks,
		ReorgsDetected:    rd.stats.ReorgsDetected,
		TransactionsFixed: rd.stats.TransactionsFixed,
		LastCheckTime:     rd.stats.LastCheckTime,
		LastReorgTime:     rd.stats.LastReorgTime,
	}
}

// parseChainType 解析链类型字符串
func (rd *ReorgDetector) parseChainType(chainStr string) pb.BlockChainType {
	switch chainStr {
	case "CHAIN_TYPE_ETHEREUM":
		return pb.BlockChainType_CHAIN_TYPE_ETHEREUM
	case "CHAIN_TYPE_BSC":
		return pb.BlockChainType_CHAIN_TYPE_BSC
	case "CHAIN_TYPE_TRON":
		return pb.BlockChainType_CHAIN_TYPE_TRON
	default:
		return pb.BlockChainType_CHAIN_TYPE_ETHEREUM
	}
}
