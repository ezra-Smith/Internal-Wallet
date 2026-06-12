package svc

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"internalwallet/proto/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

func (am *AddressMonitor) getLatestBlockWithRetry(ctx context.Context, provider Provider, chainType pb.BlockChainType) (uint64, error) {
	var latestBlock uint64

	// Prefer HeadTracker cached head (fast path).
	if am.headTracker != nil {
		if cachedBlock, exists := am.headTracker.Get(chainType); exists && cachedBlock > 0 {
			return cachedBlock, nil
		}
	}

	// 重试获取网络最新区块
	maxRetries := 3
	retryDelay := 1 * time.Second
	var err error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		networkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		latestBlock, err = provider.GetLatestBlock(networkCtx)
		cancel()

		if err == nil {
			if am.headTracker != nil {
				am.headTracker.Set(chainType, latestBlock)
			}
			return latestBlock, nil
		}

		if attempt < maxRetries {
			logx.Infof("⚠️ Failed to get latest block for chain %v (attempt %d/%d): %v, retrying...",
				chainType, attempt, maxRetries, err)

			select {
			case <-ctx.Done():
				return 0, ctx.Err()
			case <-time.After(retryDelay):
				retryDelay *= 2 // 指数退避
			}
		}
	}

	return 0, fmt.Errorf("failed to get latest block after %d attempts: %v", maxRetries, err)
}

// getLastProcessedBlockForChain 获取链的最后处理区块
// 逻辑：1. 优先使用 Redis 缓存  2. 如果无缓存，获取最新区块 - BlockLookback
func (am *AddressMonitor) getLastProcessedBlockForChain(chainType pb.BlockChainType) uint64 {
	// 1) 优先从 Redis 获取该链的“最后处理区块”
	// 与 setLastProcessedBlockForChain 保持一致（避免读写 key 不一致导致扫描游标失效）
	chainCacheKey := fmt.Sprintf("chain:last_processed_block:%s", chainType.String())
	if cachedBlock, err := am.redisCacheManager.Get(chainCacheKey); err == nil && cachedBlock != "" {
		if blockNum, parseErr := strconv.ParseUint(cachedBlock, 10, 64); parseErr == nil && blockNum > 0 {
			logx.Debugf("📂 Using cached last processed block for chain %s: %d", chainType.String(), blockNum)
			return blockNum
		}
	}

	// 2) No cursor cached: compute from head (HeadTracker cache first, provider second) with lookback.
	logx.Infof("🔄 No cached block for chain %s, fetching latest block and applying lookback...", chainType.String())

	// 获取 BlockLookback 配置
	var blockLookback uint64 = DefaultBlockLookback // 默认 20
	if am.config != nil {
		switch chainType {
		case pb.BlockChainType_CHAIN_TYPE_ETHEREUM:
			if am.config.Chains.Ethereum.BlockLookback > 0 {
				blockLookback = am.config.Chains.Ethereum.BlockLookback
			}
		case pb.BlockChainType_CHAIN_TYPE_BSC:
			if am.config.Chains.BSC.BlockLookback > 0 {
				blockLookback = am.config.Chains.BSC.BlockLookback
			}
		case pb.BlockChainType_CHAIN_TYPE_TRON:
			if am.config.Chains.Tron.BlockLookback > 0 {
				blockLookback = am.config.Chains.Tron.BlockLookback
			}
		}
	}

	// Prefer HeadTracker head to avoid an extra provider call.
	if am.headTracker != nil {
		if head, exists := am.headTracker.Get(chainType); exists && head > 0 {
			if head > blockLookback {
				return head - blockLookback
			}
			return 1
		}
	}

	// 获取 Provider 并查询最新区块
	provider, err := am.providerPool.GetProvider(chainType)
	if err != nil {
		logx.Errorf("Failed to get provider for chain %s: %v, returning 0", chainType.String(), err)
		return 0
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	latestBlock, err := provider.GetLatestBlock(ctx)
	if err != nil {
		logx.Errorf("Failed to get latest block for chain %s: %v, returning 0", chainType.String(), err)
		return 0
	}

	// 计算起始区块: 最新区块 - BlockLookback
	var startBlock uint64
	if latestBlock > blockLookback {
		startBlock = latestBlock - blockLookback
	} else {
		startBlock = 1
	}

	logx.Infof("✅ Chain %s: latest block = %d, lookback = %d, starting from block %d",
		chainType.String(), latestBlock, blockLookback, startBlock)

	return startBlock
}

// setLastProcessedBlockForChain 设置链的最后处理区块
func (am *AddressMonitor) setLastProcessedBlockForChain(chainType pb.BlockChainType, blockNumber uint64) {
	// 保存到Redis
	chainCacheKey := fmt.Sprintf("chain:last_processed_block:%s", chainType.String())

	// Cursor must be durable across restarts; do not use TTL.
	if err := am.redisCacheManager.Set(chainCacheKey, strconv.FormatUint(blockNumber, 10)); err != nil {
		logx.Errorf("Failed to cache last processed block for chain %v: %v", chainType, err)
	}
}
