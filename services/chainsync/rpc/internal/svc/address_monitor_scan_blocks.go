package svc

import (
	"context"
	"fmt"
	"time"

	"internalwallet/proto/pb"
	chainprovider "internalwallet/services/chainsync/rpc/internal/provider"

	"github.com/zeromicro/go-zero/core/logx"
)

// scanNewBlocksBatch 扫描新区块并批量过滤相关地址
func (am *AddressMonitor) scanNewBlocksBatch(ctx context.Context, provider Provider, chainType pb.BlockChainType, addressSet map[string]bool) error {
	if len(addressSet) == 0 {
		return fmt.Errorf("no addresses to monitor")
	}

	startTime := time.Now()
	// 2. 获取区块范围
	lastProcessedBlock := am.getLastProcessedBlockForChain(chainType)
	latestBlock, err := am.getLatestBlockWithRetry(ctx, provider, chainType)
	if err != nil {
		return fmt.Errorf("failed to get latest block: %v", err)
	}

	if chainType == pb.BlockChainType_CHAIN_TYPE_ETHEREUM {
		targetBlock := uint64(24281621)
		startBlock := lastProcessedBlock + 1
		endBlock := latestBlock
		scanCfg := am.getChainScanConfig(chainType)
		batchSize := scanCfg.BatchSize
		if batchSize <= 0 {
			batchSize = 100
		}
		if candidate := lastProcessedBlock + uint64(batchSize); candidate < endBlock && candidate >= lastProcessedBlock {
			endBlock = candidate
		}
		targetInRange := targetBlock >= startBlock && targetBlock <= endBlock
		// #region agent log
		writeDebugLog(map[string]interface{}{
			"sessionId":    "debug-session",
			"runId":        "pre-fix",
			"hypothesisId": "H2",
			"location":     "address_monitor.go:scanNewBlocksBatch:range",
			"message":      "scan range computed for ETH",
			"data": map[string]interface{}{
				"chain":            chainType.String(),
				"lastProcessed":    lastProcessedBlock,
				"latestBlock":      latestBlock,
				"startBlock":       startBlock,
				"endBlock":         endBlock,
				"batchSize":        batchSize,
				"targetBlock":      targetBlock,
				"targetInRange":    targetInRange,
				"monitoredAddrCnt": len(addressSet),
			},
			"timestamp": time.Now().UnixMilli(),
		})
		// #endregion
	}

	if latestBlock <= lastProcessedBlock {
		logx.Debugf("⏭️ No new blocks to scan for chain %v (last: %d, latest: %d)", chainType, lastProcessedBlock, latestBlock)
		return nil
	}

	scanCfg := am.getChainScanConfig(chainType)
	batchSize := scanCfg.BatchSize
	if batchSize <= 0 {
		batchSize = 100
	}
	// When lag is large, scale up batch size to catch up faster.
	lag := latestBlock - lastProcessedBlock
	if lag > uint64(batchSize)*5 {
		const maxCatchUpBatch = 500
		if lag < uint64(maxCatchUpBatch) {
			batchSize = int(lag)
		} else {
			batchSize = maxCatchUpBatch
		}
	}

	startBlock := lastProcessedBlock + 1
	endBlock := latestBlock
	if candidate := lastProcessedBlock + uint64(batchSize); candidate < endBlock && candidate >= lastProcessedBlock {
		endBlock = candidate
	}

	remaining := uint64(0)
	if endBlock < latestBlock {
		remaining = latestBlock - endBlock
	}

	logx.Infof("🔍 Scanning blocks %d to %d for chain %v (latest=%d, remaining=%d, batchSize=%d)",
		startBlock, endBlock, chainType, latestBlock, remaining, batchSize)

	if chainType == pb.BlockChainType_CHAIN_TYPE_ETHEREUM {
		// #region agent log
		writeDebugLog(map[string]interface{}{
			"sessionId":    "debug-session",
			"runId":        "pre-fix",
			"hypothesisId": "H7",
			"location":     "address_monitor.go:scanNewBlocksBatch:start",
			"message":      "scan batch started",
			"data": map[string]interface{}{
				"startBlock": startBlock,
				"endBlock":   endBlock,
				"latest":     latestBlock,
				"remaining":  remaining,
				"batchSize":  batchSize,
			},
			"timestamp": time.Now().UnixMilli(),
		})
		// #endregion
	}

	// 3. 按区块批量扫描
	totalBlocks := endBlock - lastProcessedBlock
	totalTransactions := 0
	totalRelevantTransactions := 0

	for blockNum := startBlock; blockNum <= endBlock; blockNum++ {
		if chainType == pb.BlockChainType_CHAIN_TYPE_ETHEREUM && (blockNum-startBlock)%50 == 0 {
			// #region agent log
			writeDebugLog(map[string]interface{}{
				"sessionId":    "debug-session",
				"runId":        "pre-fix",
				"hypothesisId": "H7",
				"location":     "address_monitor.go:scanNewBlocksBatch:progress",
				"message":      "scan progress",
				"data": map[string]interface{}{
					"currentBlock": blockNum,
					"startBlock":   startBlock,
					"endBlock":     endBlock,
				},
				"timestamp": time.Now().UnixMilli(),
			})
			// #endregion
		}
		blockTxs, relevantTxs, err := am.scanBlockWithRetry(ctx, provider, chainType, blockNum, addressSet)
		if err != nil {
			logx.Errorf("Failed to scan block %d on chain %v: %v", blockNum, chainType, err)
			if chainType == pb.BlockChainType_CHAIN_TYPE_ETHEREUM && blockNum == 24281621 {
				// #region agent log
				writeDebugLog(map[string]interface{}{
					"sessionId":    "debug-session",
					"runId":        "pre-fix",
					"hypothesisId": "H2",
					"location":     "address_monitor.go:scanNewBlocksBatch:blockError",
					"message":      "target block scan error",
					"data": map[string]interface{}{
						"blockNumber": blockNum,
						"error":       err.Error(),
					},
					"timestamp": time.Now().UnixMilli(),
				})
				// #endregion
			}
			return fmt.Errorf("scan block %d on chain %v: %w", blockNum, chainType, err)
		}

		if chainType == pb.BlockChainType_CHAIN_TYPE_ETHEREUM && blockNum == 24281621 {
			// #region agent log
			writeDebugLog(map[string]interface{}{
				"sessionId":    "debug-session",
				"runId":        "pre-fix",
				"hypothesisId": "H2",
				"location":     "address_monitor.go:scanNewBlocksBatch:targetBlock",
				"message":      "target block scanned",
				"data": map[string]interface{}{
					"blockNumber":          blockNum,
					"totalTransactions":    len(blockTxs),
					"relevantTransactions": len(relevantTxs),
				},
				"timestamp": time.Now().UnixMilli(),
			})
			// #endregion
		}

		totalTransactions += len(blockTxs)
		totalRelevantTransactions += len(relevantTxs)

		// 批量处理相关交易
		if len(relevantTxs) > 0 {
			logx.Infof("🎯 Block %d: found %d relevant transactions out of %d total", blockNum, len(relevantTxs), len(blockTxs))
			if err := am.processTransactionsBatch(relevantTxs); err != nil {
				return fmt.Errorf("process transactions for block %d on chain %v: %w", blockNum, chainType, err)
			}
		}

		// 更新链的最后处理区块
		am.setLastProcessedBlockForChain(chainType, blockNum)
	}

	logx.Infof("✅ Completed scanning %d blocks for chain %v: %d total transactions, %d relevant (latest=%d, remaining=%d)",
		totalBlocks, chainType, totalTransactions, totalRelevantTransactions, latestBlock, remaining)

	if chainType == pb.BlockChainType_CHAIN_TYPE_ETHEREUM {
		// #region agent log
		writeDebugLog(map[string]interface{}{
			"sessionId":    "debug-session",
			"runId":        "pre-fix",
			"hypothesisId": "H7",
			"location":     "address_monitor.go:scanNewBlocksBatch:end",
			"message":      "scan batch completed",
			"data": map[string]interface{}{
				"startBlock":        startBlock,
				"endBlock":          endBlock,
				"durationMs":        time.Since(startTime).Milliseconds(),
				"totalTransactions": totalTransactions,
				"relevantTxs":       totalRelevantTransactions,
			},
			"timestamp": time.Now().UnixMilli(),
		})
		// #endregion
	}

	return nil
}

// scanBlockWithRetry 扫描单个区块（带重试机制）
// 注意：为每个区块创建独立的 context，避免共享父 context 导致超时传染
func (am *AddressMonitor) scanBlockWithRetry(_ context.Context, provider Provider, chainType pb.BlockChainType, blockNumber uint64, addressSet map[string]bool) ([]*pb.Transaction, []*pb.Transaction, error) {
	var blockTxs, relevantTxs []*pb.Transaction
	var err error

	// 重试机制
	maxRetries := 3
	retryDelay := 1 * time.Second

	for attempt := 1; attempt <= maxRetries; attempt++ {
		// 为每次尝试创建独立的 context，超时 30 秒
		blockCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		blockTxs, relevantTxs, err = am.scanSingleBlock(blockCtx, provider, chainType, blockNumber, addressSet)
		cancel()

		if err == nil {
			return blockTxs, relevantTxs, nil
		}

		if attempt < maxRetries {
			logx.Infof("⚠️ Failed to scan block %d on chain %v (attempt %d/%d): %v, retrying in %v...",
				blockNumber, chainType, attempt, maxRetries, err, retryDelay)
			time.Sleep(retryDelay)
			retryDelay *= 2 // 指数退避
		}
	}

	return nil, nil, fmt.Errorf("failed to scan block %d after %d attempts: %v", blockNumber, maxRetries, err)
}

// scanSingleBlock 扫描单个区块的所有交易
func (am *AddressMonitor) scanSingleBlock(ctx context.Context, provider Provider, chainType pb.BlockChainType, blockNumber uint64, addressSet map[string]bool) ([]*pb.Transaction, []*pb.Transaction, error) {
	// 使用重试机制获取区块交易
	var transactions []*pb.Transaction
	var err error

	if chainType == pb.BlockChainType_CHAIN_TYPE_ETHEREUM || chainType == pb.BlockChainType_CHAIN_TYPE_BSC {
		ctx = chainprovider.WithSkipReceipts(ctx)
	}

	err = am.ExecuteWithRetry(ctx, fmt.Sprintf("get block transactions %d", blockNumber), func(ctx context.Context) error {
		transactions, err = provider.GetBlockTransactions(ctx, blockNumber)
		return err
	})

	if err != nil {
		return nil, nil, fmt.Errorf("failed to get block transactions for block %d: %v", blockNumber, err)
	}

	if len(transactions) == 0 {
		return []*pb.Transaction{}, []*pb.Transaction{}, nil
	}

	// 过滤相关交易
	relevantTxs := make([]*pb.Transaction, 0)
	for _, tx := range transactions {
		if am.isTransactionRelevant(chainType, tx, addressSet) {
			relevantTxs = append(relevantTxs, tx)
		}
	}

	return transactions, relevantTxs, nil
}

// isTransactionRelevant 检查交易是否涉及监控地址
func (am *AddressMonitor) isTransactionRelevant(chainType pb.BlockChainType, tx *pb.Transaction, addressSet map[string]bool) bool {
	if tx == nil {
		return false
	}

	// 使用 normalizeMonitoredAddress 保持与监控地址集合一致：
	// - EVM(ETH/BSC)：全部转小写匹配（地址大小写不敏感）
	// - TRON：保持原样（base58 大小写敏感）
	from := normalizeMonitoredAddress(chainType, tx.FromAddress)
	to := normalizeMonitoredAddress(chainType, tx.ToAddress)
	if (from != "" && addressSet[from]) || (to != "" && addressSet[to]) {
		return true
	}

	// 检查日志中的合约地址（如果是合约交互）
	for _, log := range tx.Logs {
		addr := normalizeMonitoredAddress(chainType, log.Address)
		if addr != "" && addressSet[addr] {
			return true
		}
	}
	return false
}
