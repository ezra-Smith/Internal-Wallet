package repository

import (
	"context"
	"fmt"
	"sync"
	"time"

	"internalwallet/services/chainsync/rpc/internal/model"
)

// TransactionProcessor 交易处理器接口
// 业务逻辑需要实现此接口来处理每批交易
type TransactionProcessor interface {
	ProcessTransactions(ctx context.Context, transactions []*model.UnconfirmedTransaction) error
}

// BatchProcessConfig 批量处理配置
type BatchProcessConfig struct {
	BatchSize        int           // 每批次处理数量（默认 100）
	MaxWorkers       int           // 最大并发协程数（默认 5）
	QueryTimeout     time.Duration // 查询超时时间（默认 5 秒）
	ProcessTimeout   time.Duration // 处理超时时间（默认 30 秒）
	EnableRetry      bool          // 是否启用失败重试（默认 true）
	MaxRetries       int           // 最大重试次数（默认 3）
	RetryInterval    time.Duration // 重试间隔（默认 1 秒）
	EnableCountQuery bool          // 是否先查询总数（默认 false）
}

// DefaultBatchProcessConfig 返回默认配置
func DefaultBatchProcessConfig() *BatchProcessConfig {
	return &BatchProcessConfig{
		BatchSize:        100,
		MaxWorkers:       5,
		QueryTimeout:     5 * time.Second,
		ProcessTimeout:   30 * time.Second,
		EnableRetry:      true,
		MaxRetries:       3,
		RetryInterval:    1 * time.Second,
		EnableCountQuery: false,
	}
}

// BatchProcessor 批量处理器
type BatchProcessor struct {
	repo   UnconfirmedTransactionRepository
	config *BatchProcessConfig
}

// NewBatchProcessor 创建批量处理器
func NewBatchProcessor(repo UnconfirmedTransactionRepository, config *BatchProcessConfig) *BatchProcessor {
	if config == nil {
		config = DefaultBatchProcessConfig()
	}
	return &BatchProcessor{
		repo:   repo,
		config: config,
	}
}

// ProcessTransactionsToConfirm 批量并发处理待确认的交易
// 返回：处理的总数、成功数、失败数、错误信息
func (bp *BatchProcessor) ProcessTransactionsToConfirm(
	ctx context.Context,
	chain string,
	currentBlock uint64,
	confirmations int32,
	processor TransactionProcessor,
) (total int, success int, failed int, err error) {
	// 方案 1：如果启用总数查询，先获取总数（适合数据量大的场景）
	if bp.config.EnableCountQuery {
		total, err = bp.getPendingCount(ctx, chain)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("failed to get pending count: %w", err)
		}

		// 如果没有待处理数据，直接返回
		if total == 0 {
			return 0, 0, 0, nil
		}

		return bp.processWithCount(ctx, chain, currentBlock, confirmations, total, processor)
	}

	// 方案 2：流式处理（推荐，适合大多数场景）
	// 持续查询直到没有数据为止
	return bp.processStreamStyle(ctx, chain, currentBlock, confirmations, processor)
}

// getPendingCount 获取待处理交易总数
func (bp *BatchProcessor) getPendingCount(ctx context.Context, chain string) (int, error) {
	var count int64
	queryCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	err := bp.repo.GetDB().
		WithContext(queryCtx).
		Model(&model.UnconfirmedTransaction{}).
		Where("chain = ? AND status = 0", chain).
		Count(&count).Error

	return int(count), err
}

// processWithCount 基于总数的并发批量处理
func (bp *BatchProcessor) processWithCount(
	ctx context.Context,
	chain string,
	currentBlock uint64,
	confirmations int32,
	totalCount int,
	processor TransactionProcessor,
) (total int, success int, failed int, err error) {
	// 计算需要分多少批
	batchCount := (totalCount + bp.config.BatchSize - 1) / bp.config.BatchSize

	// 创建任务通道
	tasks := make(chan int, batchCount)
	results := make(chan *BatchResult, batchCount)

	// 启动结果收集器
	var wg sync.WaitGroup
	resultCollector := &ResultCollector{
		TotalBatches: batchCount,
		Results:      make([]*BatchResult, 0, batchCount),
	}

	// 启动 worker pool
	workerPool := NewWorkerPool(bp.config.MaxWorkers, bp.config, bp.repo, processor)

	// 分配任务
	for i := 0; i < batchCount; i++ {
		tasks <- i
	}
	close(tasks)

	// 启动 worker 处理任务
	wg.Add(bp.config.MaxWorkers)
	for i := 0; i < bp.config.MaxWorkers; i++ {
		go func(workerID int) {
			defer wg.Done()
			workerPool.ProcessWorker(ctx, workerID, tasks, results, chain, currentBlock, confirmations)
		}(i)
	}

	// 等待所有 worker 完成
	go func() {
		wg.Wait()
		close(results)
	}()

	// 收集结果
	for result := range results {
		resultCollector.AddResult(result)
	}

	// 统计结果
	return resultCollector.GetSummary()
}

// processStreamStyle 流式处理（推荐方案）
// 持续分批查询数据，直到没有数据为止
func (bp *BatchProcessor) processStreamStyle(
	ctx context.Context,
	chain string,
	currentBlock uint64,
	confirmations int32,
	processor TransactionProcessor,
) (total int, success int, failed int, err error) {
	var lastID int64 = 0
	var mutex sync.Mutex

	for {
		// 查询一批数据
		batch, err := bp.queryNextBatch(ctx, chain, currentBlock, confirmations, lastID)
		if err != nil {
			return total, success, failed, fmt.Errorf("failed to query batch: %w", err)
		}

		// 没有更多数据
		if len(batch) == 0 {
			break
		}

		// 处理这一批数据
		batchSuccess, batchFailed := bp.processSingleBatch(ctx, batch, processor)

		// 更新统计
		mutex.Lock()
		total += len(batch)
		success += batchSuccess
		failed += batchFailed

		// 更新 lastID（用于下一批查询）
		for _, tx := range batch {
			if tx.ID > lastID {
				lastID = tx.ID
			}
		}
		mutex.Unlock()

		// 如果返回的数据少于批次大小，说明已经是最后一批了
		if len(batch) < bp.config.BatchSize {
			break
		}

		// 避免无限循环（安全保护）
		if total >= 10000 { // 最多处理 1 万条
			break
		}
	}

	return total, success, failed, nil
}

// queryNextBatch 查询下一批数据
func (bp *BatchProcessor) queryNextBatch(
	ctx context.Context,
	chain string,
	currentBlock uint64,
	confirmations int32,
	lastID int64,
) ([]*model.UnconfirmedTransaction, error) {
	queryCtx, cancel := context.WithTimeout(ctx, bp.config.QueryTimeout)
	defer cancel()

	minBlock, ok := minBlockForConfirmations(currentBlock, confirmations)
	if !ok {
		return []*model.UnconfirmedTransaction{}, nil
	}

	var transactions []*model.UnconfirmedTransaction
	query := bp.repo.GetDB().
		WithContext(queryCtx).
		Select("id, tx_hash, chain, block_number, block_hash, from_address, to_address, monitored_address, source, direction, counterparty_address, monitored_is_internal, counterparty_is_internal, counterparty_source_bits, value, gas_price, gas_used, gas_fee, energy_used, bandwidth_used, transaction_index, block_timestamp, log_index, transaction_type, token_address, token_name, token_symbol, token_decimals, token_amount, exec_status, exec_checked_at, exec_next_check_at, exec_error_message, status, confirmations, required_confirmations, message_id, message_topic, sent_at, error_message, retry_count, max_retries, created_at, updated_at, deleted_at").
		Where("chain = ? AND status = 0 AND block_number <= ?", chain, minBlock).
		Order("id ASC").
		Limit(bp.config.BatchSize)

	if lastID > 0 {
		query = query.Where("id > ?", lastID)
	}

	err := query.Find(&transactions).Error
	return transactions, err
}

// processSingleBatch 处理单批数据（带重试）
func (bp *BatchProcessor) processSingleBatch(
	ctx context.Context,
	batch []*model.UnconfirmedTransaction,
	processor TransactionProcessor,
) (success int, failed int) {
	var err error

	// 尝试处理（带重试）
	for attempt := 0; attempt <= bp.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// 重试前等待
			time.Sleep(bp.config.RetryInterval * time.Duration(attempt))
		}

		// 创建处理超时上下文
		processCtx, cancel := context.WithTimeout(ctx, bp.config.ProcessTimeout)

		// 处理这批数据
		err = processor.ProcessTransactions(processCtx, batch)
		cancel()
		if err == nil {
			// 成功
			return len(batch), 0
		}

		// 如果不是重试启用，直接失败
		if !bp.config.EnableRetry {
			break
		}

		// 如果是上下文取消或超时，不再重试
		if ctx.Err() != nil || processCtx.Err() != nil {
			err = fmt.Errorf("processing timeout: %w", err)
			break
		}
	}

	// 所有重试都失败
	return 0, len(batch)
}

// BatchResult 批处理结果
type BatchResult struct {
	BatchID  int
	Success  int
	Failed   int
	Error    error
	Duration time.Duration
}

// ResultCollector 结果收集器
type ResultCollector struct {
	TotalBatches int
	Results      []*BatchResult
	mutex        sync.Mutex
}

// AddResult 添加结果
func (rc *ResultCollector) AddResult(result *BatchResult) {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()
	rc.Results = append(rc.Results, result)
}

// GetSummary 获取汇总结果
func (rc *ResultCollector) GetSummary() (total int, success int, failed int, err error) {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()

	for _, result := range rc.Results {
		success += result.Success
		failed += result.Failed
		if result.Error != nil {
			err = fmt.Errorf("batch %d failed: %w", result.BatchID, result.Error)
		}
	}

	total = success + failed
	return total, success, failed, err
}

// WorkerPool worker池
type WorkerPool struct {
	maxWorkers int
	config     *BatchProcessConfig
	repo       UnconfirmedTransactionRepository
	processor  TransactionProcessor
}

// NewWorkerPool 创建worker池
func NewWorkerPool(maxWorkers int, config *BatchProcessConfig, repo UnconfirmedTransactionRepository, processor TransactionProcessor) *WorkerPool {
	return &WorkerPool{
		maxWorkers: maxWorkers,
		config:     config,
		repo:       repo,
		processor:  processor,
	}
}

// ProcessWorker worker处理任务
func (wp *WorkerPool) ProcessWorker(
	ctx context.Context,
	workerID int,
	tasks <-chan int,
	results chan<- *BatchResult,
	chain string,
	currentBlock uint64,
	confirmations int32,
) {
	for batchID := range tasks {
		startTime := time.Now()

		// 计算这一批的偏移量
		offset := batchID * wp.config.BatchSize

		// 查询这一批数据
		queryCtx, cancel := context.WithTimeout(ctx, wp.config.QueryTimeout)
		transactions, err := wp.queryBatchByOffset(queryCtx, chain, currentBlock, confirmations, offset)
		cancel()

		if err != nil {
			results <- &BatchResult{
				BatchID: batchID,
				Success: 0,
				Failed:  0,
				Error:   fmt.Errorf("worker %d: failed to query batch %d: %w", workerID, batchID, err),
			}
			continue
		}

		// 如果没有数据，跳过
		if len(transactions) == 0 {
			results <- &BatchResult{
				BatchID:  batchID,
				Success:  0,
				Failed:   0,
				Error:    nil,
				Duration: time.Since(startTime),
			}
			continue
		}

		// 处理这批数据
		success, failed := wp.processBatchWithRetry(ctx, transactions)

		results <- &BatchResult{
			BatchID:  batchID,
			Success:  success,
			Failed:   failed,
			Error:    nil,
			Duration: time.Since(startTime),
		}
	}
}

// queryBatchByOffset 按偏移量查询批次（用于并发场景）
func (wp *WorkerPool) queryBatchByOffset(
	ctx context.Context,
	chain string,
	currentBlock uint64,
	confirmations int32,
	offset int,
) ([]*model.UnconfirmedTransaction, error) {
	minBlock, ok := minBlockForConfirmations(currentBlock, confirmations)
	if !ok {
		return []*model.UnconfirmedTransaction{}, nil
	}

	var transactions []*model.UnconfirmedTransaction
	err := wp.repo.GetDB().
		WithContext(ctx).
		Select("id, tx_hash, chain, block_number, block_hash, from_address, to_address, monitored_address, source, direction, counterparty_address, monitored_is_internal, counterparty_is_internal, counterparty_source_bits, value, gas_price, gas_used, gas_fee, energy_used, bandwidth_used, transaction_index, block_timestamp, log_index, transaction_type, token_address, token_name, token_symbol, token_decimals, token_amount, exec_status, exec_checked_at, exec_next_check_at, exec_error_message, status, confirmations, required_confirmations, message_id, message_topic, sent_at, error_message, retry_count, max_retries, created_at, updated_at, deleted_at").
		Where("chain = ? AND status = 0 AND block_number <= ?", chain, minBlock).
		Order("id ASC").
		Offset(offset).
		Limit(wp.config.BatchSize).
		Find(&transactions).Error

	return transactions, err
}

// processBatchWithRetry 处理批次（带重试）
func (wp *WorkerPool) processBatchWithRetry(
	ctx context.Context,
	transactions []*model.UnconfirmedTransaction,
) (success int, failed int) {
	var err error

	for attempt := 0; attempt <= wp.config.MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(wp.config.RetryInterval * time.Duration(attempt))
		}

		processCtx, cancel := context.WithTimeout(ctx, wp.config.ProcessTimeout)

		err = wp.processor.ProcessTransactions(processCtx, transactions)
		cancel()

		if err == nil {
			return len(transactions), 0
		}

		if !wp.config.EnableRetry {
			break
		}
	}

	return 0, len(transactions)
}

// ProcessTransactionsConcurrent 简化版并发处理（推荐使用）
// 自动使用流式处理，无需预先知道总数
func ProcessTransactionsConcurrent(
	ctx context.Context,
	repo UnconfirmedTransactionRepository,
	chain string,
	currentBlock uint64,
	confirmations int32,
	processor TransactionProcessor,
	config *BatchProcessConfig,
) (total int, success int, failed int, err error) {
	if config == nil {
		config = DefaultBatchProcessConfig()
	}

	bp := NewBatchProcessor(repo, config)
	return bp.ProcessTransactionsToConfirm(ctx, chain, currentBlock, confirmations, processor)
}
