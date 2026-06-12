package evm

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
)

func (p *Web3Provider) batchScanAddressTransactions(ctx context.Context, address common.Address, startBlock, endBlock uint64, pageSize int, config BatchConfig) ([]*pb.Transaction, error) {
	totalBlocks := endBlock - startBlock + 1
	if totalBlocks == 0 {
		return nil, fmt.Errorf("invalid block range: %d - %d", startBlock, endBlock)
	}

	logx.Infof("🚀 Starting batch scan for address %s, blocks %d to %d, total %d blocks",
		address.Hex(), startBlock, endBlock, totalBlocks)

	batches := p.createBatches(startBlock, endBlock, config.MaxBatchSize)
	batchCount := len(batches)

	logx.Infof("📦 Divided into %d batches, max %d blocks per batch", batchCount, config.MaxBatchSize)

	semaphore := make(chan struct{}, config.MaxConcurrency)
	var (
		wg              sync.WaitGroup
		mu              sync.Mutex
		allTransactions []*pb.Transaction
		firstError      error
	)

	for i, batch := range batches {
		mu.Lock()
		if firstError != nil || len(allTransactions) >= pageSize {
			mu.Unlock()
			break
		}
		mu.Unlock()

		wg.Add(1)
		go func(batchIndex int, batch BatchRange) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			batchTxs, err := p.processBatch(ctx, address, batch, config)
			if err != nil {
				mu.Lock()
				if firstError == nil {
					firstError = err
				}
				mu.Unlock()
				logx.Errorf("❌ Batch %d processing failed: %v", batchIndex+1, err)
				return
			}

			mu.Lock()
			allTransactions = append(allTransactions, batchTxs...)
			mu.Unlock()

			logx.Debugf("✅ Batch %d/%d completed, found %d transactions", batchIndex+1, batchCount, len(batchTxs))
		}(i, batch)
	}

	wg.Wait()

	if firstError != nil {
		return nil, fmt.Errorf("batch scan failed: %v", firstError)
	}

	if len(allTransactions) > pageSize {
		allTransactions = allTransactions[:pageSize]
	}

	logx.Infof("🎉 Batch scan completed! Found %d transactions", len(allTransactions))
	return allTransactions, nil
}

func (p *Web3Provider) createBatches(startBlock, endBlock uint64, maxBatchSize int) []BatchRange {
	var batches []BatchRange
	batchSize := uint64(maxBatchSize)

	for start := startBlock; start <= endBlock; start += batchSize {
		end := start + batchSize - 1
		if end > endBlock {
			end = endBlock
		}
		batches = append(batches, BatchRange{Start: start, End: end})
	}

	return batches
}

func (p *Web3Provider) processBatch(ctx context.Context, address common.Address, batch BatchRange, config BatchConfig) ([]*pb.Transaction, error) {
	batchCtx, cancel := context.WithTimeout(context.Background(), config.BatchTimeout)
	defer cancel()

	for attempt := 0; attempt < config.RetryAttempts; attempt++ {
		if attempt > 0 {
			logx.Infof("Batch %d-%d retry attempt %d", batch.Start, batch.End, attempt)
			time.Sleep(time.Duration(attempt) * time.Second)
		}

		txs, err := p.processBatchInternal(batchCtx, address, batch)
		if err == nil {
			return txs, nil
		}

		if batchCtx.Err() != nil {
			return nil, batchCtx.Err()
		}

		logx.Infof("Batch %d-%d processing failed: %v", batch.Start, batch.End, err)
	}

	return nil, fmt.Errorf("batch %d-%d failed after %d retry attempts", batch.Start, batch.End, config.RetryAttempts)
}

func (p *Web3Provider) processBatchInternal(ctx context.Context, address common.Address, batch BatchRange) ([]*pb.Transaction, error) {
	transactions := make([]*pb.Transaction, 0)
	blocks := make([]*types.Block, 0, batch.End-batch.Start+1)

	for num := batch.Start; num <= batch.End; num++ {
		block, err := p.getBlockWithRetry(ctx, num)
		if err != nil {
			return nil, fmt.Errorf("failed to get block %d: %v", num, err)
		}
		blocks = append(blocks, block)
	}

	for _, block := range blocks {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			blockTxs := p.scanBlockForAddress(block, address)
			transactions = append(transactions, blockTxs...)
		}
	}

	return transactions, nil
}

func (p *Web3Provider) getBlockWithRetry(ctx context.Context, blockNum uint64) (*types.Block, error) {
	const maxRetries = 3

	for attempt := 0; attempt < maxRetries; attempt++ {
		blockCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		block, err := p.client.BlockByNumber(blockCtx, big.NewInt(int64(blockNum)))
		cancel()

		if err == nil {
			return block, nil
		}

		logx.Infof("Failed to get block %d (attempt %d/%d): %v", blockNum, attempt+1, maxRetries, err)
		if attempt < maxRetries-1 {
			waitTime := time.Duration(attempt+1) * time.Second
			logx.Infof("Waiting %v before retrying block %d", waitTime, blockNum)
			time.Sleep(waitTime)
		}
	}

	return nil, fmt.Errorf("failed to get block %d after %d attempts", blockNum, maxRetries)
}

