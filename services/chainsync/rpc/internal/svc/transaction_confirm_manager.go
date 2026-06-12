package svc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/chainsync/rpc/internal/config"
	"internalwallet/services/chainsync/rpc/internal/model"
	"internalwallet/services/chainsync/rpc/internal/provider"
	"internalwallet/services/chainsync/rpc/internal/repository"

	"github.com/zeromicro/go-zero/core/logx"
)

const (
	// Exec status recheck throttling (robust mode)
	defaultExecRecheckDelay      = 30 * time.Second
	defaultExecRecheckDelayOnErr = 60 * time.Second
)

type tronGridTxInfoResp struct {
	Result  string `json:"result"` // "SUCCESS" / "FAILED"
	Receipt struct {
		Result string `json:"result"` // "SUCCESS" / "REVERT" / "OUT_OF_ENERGY" etc.
	} `json:"receipt"`
}

// TransactionConfirmManager 交易确认管理器
type TransactionConfirmManager struct {
	unconfirmedTxRepo repository.UnconfirmedTransactionRepository
	providerPool      *provider.Pool
	headTracker       *HeadTracker

	// 停止信号
	stopCh chan struct{}
	wg     sync.WaitGroup

	// 配置
	config          *config.Config         // 配置信息
	checkInterval   time.Duration          // 检查间隔
	batchSize       int                    // 批量处理大小
	cleanupInterval time.Duration          // 清理间隔
	kafkaProducer   KafkaProducerInterface // Kafka生产者接口

	// 区块重组检测器
	reorgDetector *ReorgDetector

	// Kafka重试队列
	kafkaRetryQueue *KafkaRetryQueue
}

// NewTransactionConfirmManager 创建交易确认管理器
func NewTransactionConfirmManager(
	unconfirmedTxRepo repository.UnconfirmedTransactionRepository,
	providerPool *provider.Pool,
	headTracker *HeadTracker,
	kafkaProducer KafkaProducerInterface,
) *TransactionConfirmManager {
	return &TransactionConfirmManager{
		unconfirmedTxRepo: unconfirmedTxRepo,
		providerPool:      providerPool,
		headTracker:       headTracker,
		stopCh:            make(chan struct{}),
		checkInterval:     time.Duration(DefaultTransactionConfirmCheckIntervalSeconds) * time.Second,
		cleanupInterval:   time.Duration(DefaultTransactionCleanupIntervalHours) * time.Hour,
		batchSize:         DefaultTransactionConfirmBatchSize,
		kafkaProducer:     kafkaProducer,
	}
}

// NewTransactionConfirmManagerWithConfig 根据配置创建交易确认管理器
func NewTransactionConfirmManagerWithConfig(
	unconfirmedTxRepo repository.UnconfirmedTransactionRepository,
	providerPool *provider.Pool,
	headTracker *HeadTracker,
	kafkaProducer KafkaProducerInterface,
	config *config.Config,
	kafkaRetryQueue *KafkaRetryQueue,
) *TransactionConfirmManager {
	tcm := &TransactionConfirmManager{
		unconfirmedTxRepo: unconfirmedTxRepo,
		providerPool:      providerPool,
		headTracker:       headTracker,
		config:            config,
		stopCh:            make(chan struct{}),
		batchSize:         DefaultTransactionConfirmBatchSize,
		kafkaProducer:     kafkaProducer,
		kafkaRetryQueue:   kafkaRetryQueue,
	}

	// 根据配置设置检查间隔和清理间隔
	if config != nil && config.Monitoring.TransactionConfirmation != nil {
		tcm.checkInterval = time.Duration(config.Monitoring.TransactionConfirmation.CheckInterval) * time.Second
		tcm.cleanupInterval = time.Duration(config.Monitoring.TransactionConfirmation.CleanupInterval) * time.Second
	} else {
		tcm.checkInterval = time.Duration(DefaultTransactionConfirmCheckIntervalSeconds) * time.Second
		tcm.cleanupInterval = time.Duration(DefaultTransactionCleanupIntervalHours) * time.Hour
	}

	// 创建区块重组检测器
	tcm.reorgDetector = NewReorgDetector(providerPool, unconfirmedTxRepo, kafkaProducer, kafkaRetryQueue)

	return tcm
}

// Start 启动交易确认管理器
func (tcm *TransactionConfirmManager) Start() {
	select {
	case <-tcm.stopCh:
		logx.Info("⏭️ Transaction confirm manager start skipped: already stopped")
		return
	default:
	}

	logx.Info("🚀 Starting transaction confirm manager...")

	// 启动主循环
	tcm.wg.Add(1)
	go tcm.confirmLoop()

	// 启动清理任务
	tcm.wg.Add(1)
	go tcm.cleanupLoop()

	// 启动Kafka重试队列
	if tcm.kafkaRetryQueue != nil {
		tcm.kafkaRetryQueue.Start()
	}

	logx.Info("✅ Transaction confirm manager started")
}

// Stop 停止交易确认管理器
func (tcm *TransactionConfirmManager) Stop() {
	logx.Info("Stopping transaction confirm manager...")

	close(tcm.stopCh)
	tcm.wg.Wait()

	// 停止Kafka重试队列
	if tcm.kafkaRetryQueue != nil {
		tcm.kafkaRetryQueue.Stop()
	}

	// Kafka producer is owned by ServiceContext; do not close it here.

	logx.Info("✅ Transaction confirm manager stopped")
}

// confirmLoop 确认循环
func (tcm *TransactionConfirmManager) confirmLoop() {
	defer tcm.wg.Done()

	if tcm.checkInterval <= 0 {
		tcm.checkInterval = time.Duration(DefaultTransactionConfirmCheckIntervalSeconds) * time.Second
	}
	ticker := time.NewTicker(tcm.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-tcm.stopCh:
			logx.Info("Transaction confirm loop stopped")
			return

		case <-ticker.C:
			tcm.checkAndConfirmTransactions()
		}
	}
}

// cleanupLoop 清理循环
func (tcm *TransactionConfirmManager) cleanupLoop() {
	defer tcm.wg.Done()

	// 使用配置的间隔，默认为24小时
	cleanupInterval := tcm.cleanupInterval
	if cleanupInterval <= 0 {
		cleanupInterval = time.Duration(DefaultTransactionCleanupIntervalHours) * time.Hour
	}

	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	// 启动时先执行一次清理
	tcm.cleanupExpiredTransactions()

	for {
		select {
		case <-tcm.stopCh:
			logx.Info("Transaction cleanup loop stopped")
			return

		case <-ticker.C:
			tcm.cleanupExpiredTransactions()
		}
	}
}

// checkAndConfirmTransactions 检查并确认交易（使用批量并发处理）
func (tcm *TransactionConfirmManager) checkAndConfirmTransactions() {
	// 获取当前各链的最新区块
	chains := []string{"CHAIN_TYPE_ETHEREUM", "CHAIN_TYPE_BSC", "CHAIN_TYPE_TRON"}
	currentBlocks := make(map[string]uint64)

	for _, chain := range chains {
		// 从HeadTracker获取缓存的最新区块
		if tcm.headTracker != nil {
			if block, exists := tcm.headTracker.Get(pb.BlockChainType(pb.BlockChainType_value[chain])); exists {
				currentBlocks[chain] = block
			}
		}
	}

	// 处理每个链的未确认交易
	for _, chain := range chains {
		currentBlock, exists := currentBlocks[chain]
		if !exists {
			logx.Infof("No current block for chain %s, skipping", chain)
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(DefaultTransactionConfirmContextTimeout)*time.Second)
		// 使用批量并发处理器处理交易
		total, success, failed, err := repository.ProcessTransactionsConcurrent(
			ctx,
			tcm.unconfirmedTxRepo,
			chain,
			currentBlock,
			0,   // confirmations (0 表示所有待确认的交易)
			tcm, // 实现 TransactionProcessor 接口
			&repository.BatchProcessConfig{
				BatchSize:        500, // 每批处理 500 条
				MaxWorkers:       5,   // 最多 5 个并发协程
				QueryTimeout:     5 * time.Second,
				ProcessTimeout:   30 * time.Second,
				EnableRetry:      true,
				MaxRetries:       3,
				RetryInterval:    1 * time.Second,
				EnableCountQuery: false, // 使用流式处理（推荐）
			},
		)
		cancel()

		if err != nil {
			logx.Errorf("Failed to process transactions for chain %s: %v", chain, err)
			continue
		}

		if total > 0 {
			logx.Infof("🔍 Chain %s (current block: %d) - Processed %d transactions: %d success, %d failed",
				chain, currentBlock, total, success, failed)
		}
	}
}

// ProcessTransactions 实现 TransactionProcessor 接口 - 批量处理交易
func (tcm *TransactionConfirmManager) ProcessTransactions(ctx context.Context, transactions []*model.UnconfirmedTransaction) error {
	// 获取当前区块（从 HeadTracker）
	if len(transactions) == 0 {
		return nil
	}

	// 确定链类型
	chainType := pb.BlockChainType(pb.BlockChainType_value[transactions[0].Chain])

	var currentBlock uint64
	var exists bool
	if tcm.headTracker != nil {
		currentBlock, exists = tcm.headTracker.Get(chainType)
	}
	if !exists {
		// 如果获取不到，使用最大区块号 + 默认确认数
		var maxBlock uint64 = 0
		var defaultConfirmations int32 = 12 // 默认值

		// 从配置中获取默认确认数
		if tcm.config != nil {
			switch chainType {
			case pb.BlockChainType_CHAIN_TYPE_ETHEREUM:
				defaultConfirmations = int32(tcm.config.Chains.Ethereum.Confirmations)
			case pb.BlockChainType_CHAIN_TYPE_BSC:
				defaultConfirmations = int32(tcm.config.Chains.BSC.Confirmations)
			case pb.BlockChainType_CHAIN_TYPE_TRON:
				defaultConfirmations = int32(tcm.config.Chains.Tron.Confirmations)
			}
		}

		for _, tx := range transactions {
			if tx.BlockNumber > maxBlock {
				maxBlock = tx.BlockNumber
			}
		}
		currentBlock = maxBlock + uint64(defaultConfirmations)
	}

	return tcm.processTransactions(ctx, transactions, currentBlock)
}

// processTransactions 批量处理交易（内部方法）
func (tcm *TransactionConfirmManager) processTransactions(ctx context.Context, transactions []*model.UnconfirmedTransaction, currentBlock uint64) error {
	var lastErr error

	for _, tx := range transactions {
		// ⚠️ 批量处理时跳过区块验证，避免超时
		// 原因：每个交易都调用 VerifyTransactionBlock 会导致大量 RPC 调用，在高并发时容易超时
		// 区块重组检测由其他专门的定时任务处理
		//
		// if tcm.reorgDetector != nil {
		//     isValid, newBlockHash, err := tcm.reorgDetector.VerifyTransactionBlock(ctx, tx)
		//     if err != nil {
		//         logx.Errorf("Failed to verify block for transaction %s: %v", tx.TxHash, err)
		//         lastErr = err
		//         continue
		//     }
		//     if !isValid {
		//         logx.Infof("⚠️ Block reorg detected for transaction %s, handling...", tx.TxHash)
		//         if handleErr := tcm.reorgDetector.HandleReorgedTransaction(ctx, tx, newBlockHash); handleErr != nil {
		//             logx.Errorf("Failed to handle reorged transaction %s: %v", tx.TxHash, handleErr)
		//             lastErr = handleErr
		//         }
		//         continue
		//     }
		// }

		// 计算确认数
		confirmations := int32(0)
		if currentBlock >= tx.BlockNumber {
			confirmations = int32(currentBlock - tx.BlockNumber)
		}
		tx.Confirmations = confirmations

		// 检查是否有足够的确认数
		if confirmations >= tx.RequiredConfirmations {
			// IMPORTANT: Persist the latest confirmations before sending.
			// Otherwise, once status is marked as "sent" (status=1), the confirm loop no longer updates
			// confirmations (it only scans status=0). This can cause UI/ops views backed by
			// `unconfirmed_transaction.confirmations` to appear "stuck confirming" forever.
			if err := tcm.unconfirmedTxRepo.UpdateConfirmations(ctx, uint64(tx.ID), confirmations); err != nil {
				logx.Errorf("Failed to update confirmations for transaction %s before kafka send: %v", tx.TxHash, err)
				// Best-effort: do not block confirmation pipeline on this.
			}

			// Robust gate: verify on-chain execution success before producing Kafka message.
			ok, status, verifyErr := tcm.verifyAndUpdateExecutionStatus(ctx, tx)
			if verifyErr != nil {
				logx.Errorf("Failed to verify exec status for tx %s: %v", tx.TxHash, verifyErr)
				lastErr = verifyErr
				continue
			}
			if !ok {
				// Not eligible to send (unknown/pending or failed).
				if status == pb.TransactionStatus_TRANSACTION_STATUS_FAILED {
					logx.Infof("⛔️ Skip Kafka send for failed tx: %s", tx.TxHash)
				}
				continue
			}

			// 发送到Kafka
			if err := tcm.sendToKafka(tx); err != nil {
				logx.Errorf("Failed to send transaction %s to kafka: %v", tx.TxHash, err)

				// 将失败消息加入重试队列
				if tcm.kafkaRetryQueue != nil {
					tcm.enqueueFailedKafkaMessage(ctx, tx, err)
				}

				// 标记为失败
				if markErr := tcm.unconfirmedTxRepo.MarkAsFailed(ctx, uint64(tx.ID), err.Error()); markErr != nil {
					logx.Errorf("Failed to mark transaction %s as failed: %v", tx.TxHash, markErr)
				}
				lastErr = err
				continue
			}

			// 标记为已发送
			if err := tcm.unconfirmedTxRepo.MarkAsSent(ctx, uint64(tx.ID), tx.MessageID, tx.MessageTopic); err != nil {
				logx.Errorf("Failed to mark transaction %s as sent: %v", tx.TxHash, err)
				lastErr = err
			} else {
				logx.Infof("✅ Transaction %s confirmed and sent to kafka (confirmations: %d/%d)",
					tx.TxHash, confirmations, tx.RequiredConfirmations)
			}
		} else {
			// 更新确认数
			if err := tcm.unconfirmedTxRepo.UpdateConfirmations(ctx, uint64(tx.ID), confirmations); err != nil {
				logx.Errorf("Failed to update confirmations for transaction %s: %v", tx.TxHash, err)
				lastErr = err
			}
		}
	}

	// 如果有错误，返回最后一个错误
	return lastErr
}

// verifyAndUpdateExecutionStatus checks authoritative on-chain receipt/txInfo (best-effort) and updates DB.
// Returns ok=true only when the tx is confirmed(success) and eligible to be sent to Kafka.
func (tcm *TransactionConfirmManager) verifyAndUpdateExecutionStatus(ctx context.Context, tx *model.UnconfirmedTransaction) (ok bool, status pb.TransactionStatus, err error) {
	if tx == nil {
		return false, pb.TransactionStatus_TRANSACTION_STATUS_UNSPECIFIED, nil
	}
	if tcm == nil || tcm.unconfirmedTxRepo == nil || tcm.providerPool == nil {
		return false, pb.TransactionStatus_TRANSACTION_STATUS_UNSPECIFIED, fmt.Errorf("dependencies not configured")
	}

	// Short-circuit terminal failed.
	if tx.ExecStatus == model.ExecStatusFailed {
		return false, pb.TransactionStatus_TRANSACTION_STATUS_FAILED, nil
	}

	now := time.Now().Local()
	if tx.ExecNextCheckAt != nil && now.Before(*tx.ExecNextCheckAt) {
		return false, pb.TransactionStatus_TRANSACTION_STATUS_PENDING, nil
	}

	chainType := pb.BlockChainType(pb.BlockChainType_value[tx.Chain])
	provider, pErr := tcm.providerPool.GetProvider(chainType)
	if pErr != nil || provider == nil {
		next := now.Add(defaultExecRecheckDelayOnErr)
		_ = tcm.unconfirmedTxRepo.UpdateExecStatus(ctx, uint64(tx.ID), model.ExecStatusUnknown, now, &next, "provider not available")
		return false, pb.TransactionStatus_TRANSACTION_STATUS_UNSPECIFIED, fmt.Errorf("get provider failed: %w", pErr)
	}

	fullTx, gtErr := provider.GetTransaction(ctx, tx.TxHash)
	if gtErr != nil || fullTx == nil {
		next := now.Add(defaultExecRecheckDelayOnErr)
		msg := ""
		if gtErr != nil {
			msg = gtErr.Error()
		} else {
			msg = "nil tx"
		}
		_ = tcm.unconfirmedTxRepo.UpdateExecStatus(ctx, uint64(tx.ID), model.ExecStatusUnknown, now, &next, msg)
		return false, pb.TransactionStatus_TRANSACTION_STATUS_UNSPECIFIED, nil
	}

	status = fullTx.Status
	// TRON fallback: some providers (especially self-hosted proxies) may return PENDING when txInfo is unavailable.
	// Use TronGrid as authoritative fallback to avoid leaving failed txs in unknown forever.
	fallbackReason := ""
	if chainType == pb.BlockChainType_CHAIN_TYPE_TRON &&
		(status == pb.TransactionStatus_TRANSACTION_STATUS_PENDING || status == pb.TransactionStatus_TRANSACTION_STATUS_UNSPECIFIED) {
		if fgStatus, reason, ok := tronGridFallbackStatusAndReason(ctx, tx.TxHash); ok {
			status = fgStatus
			fallbackReason = reason
		}
	}
	switch status {
	case pb.TransactionStatus_TRANSACTION_STATUS_CONFIRMED:
		if uErr := tcm.unconfirmedTxRepo.UpdateExecStatus(ctx, uint64(tx.ID), model.ExecStatusConfirmed, now, nil, ""); uErr != nil {
			return false, status, uErr
		}
		tx.ExecStatus = model.ExecStatusConfirmed
		tx.ExecCheckedAt = &now
		tx.ExecNextCheckAt = nil
		tx.ExecErrorMessage = ""
		return true, status, nil
	case pb.TransactionStatus_TRANSACTION_STATUS_FAILED:
		reason := strings.TrimSpace(fallbackReason)
		if reason == "" {
			reason = strings.TrimSpace(fullTx.Status.String())
		}
		if reason == "" {
			reason = "exec failed"
		}
		if uErr := tcm.unconfirmedTxRepo.MarkAsExecFailedTerminal(ctx, uint64(tx.ID), reason); uErr != nil {
			return false, status, uErr
		}
		tx.ExecStatus = model.ExecStatusFailed
		tx.ExecCheckedAt = &now
		tx.ExecNextCheckAt = nil
		tx.ExecErrorMessage = reason
		// Do not treat as error; it's a valid terminal outcome.
		return false, status, nil
	default:
		// Still pending/unknown: defer next check.
		next := now.Add(defaultExecRecheckDelay)
		_ = tcm.unconfirmedTxRepo.UpdateExecStatus(ctx, uint64(tx.ID), model.ExecStatusUnknown, now, &next, status.String())
		tx.ExecStatus = model.ExecStatusUnknown
		tx.ExecCheckedAt = &now
		tx.ExecNextCheckAt = &next
		tx.ExecErrorMessage = status.String()
		return false, status, nil
	}
}

func tronGridFallbackStatusAndReason(ctx context.Context, txHash string) (pb.TransactionStatus, string, bool) {
	txHash = strings.TrimSpace(txHash)
	if txHash == "" {
		return pb.TransactionStatus_TRANSACTION_STATUS_UNSPECIFIED, "", false
	}
	callCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	body := []byte(fmt.Sprintf(`{"value":"%s"}`, txHash))
	req, err := http.NewRequestWithContext(callCtx, http.MethodPost, "https://api.trongrid.io/wallet/gettransactioninfobyid", bytes.NewReader(body))
	if err != nil {
		return pb.TransactionStatus_TRANSACTION_STATUS_UNSPECIFIED, "", false
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return pb.TransactionStatus_TRANSACTION_STATUS_UNSPECIFIED, "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.ReadAll(io.LimitReader(resp.Body, 1024))
		return pb.TransactionStatus_TRANSACTION_STATUS_UNSPECIFIED, "", false
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return pb.TransactionStatus_TRANSACTION_STATUS_UNSPECIFIED, "", false
	}
	var info tronGridTxInfoResp
	if err := json.Unmarshal(raw, &info); err != nil {
		return pb.TransactionStatus_TRANSACTION_STATUS_UNSPECIFIED, "", false
	}

	receipt := strings.ToUpper(strings.TrimSpace(info.Receipt.Result))
	switch receipt {
	case "SUCCESS":
		return pb.TransactionStatus_TRANSACTION_STATUS_CONFIRMED, "trongrid:SUCCESS", true
	case "REVERT", "FAILED", "OUT_OF_ENERGY":
		return pb.TransactionStatus_TRANSACTION_STATUS_FAILED, "trongrid:" + receipt, true
	default:
		top := strings.ToUpper(strings.TrimSpace(info.Result))
		if top == "SUCCESS" {
			return pb.TransactionStatus_TRANSACTION_STATUS_CONFIRMED, "trongrid:SUCCESS", true
		}
		if top == "FAILED" {
			return pb.TransactionStatus_TRANSACTION_STATUS_FAILED, "trongrid:FAILED", true
		}
		return pb.TransactionStatus_TRANSACTION_STATUS_UNSPECIFIED, "", false
	}
}

// cleanupExpiredTransactions 清理过期的交易
func (tcm *TransactionConfirmManager) cleanupExpiredTransactions() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(DefaultTransactionConfirmContextTimeout)*time.Second)
	defer cancel()

	// 删除过期的未确认交易
	before := time.Now().AddDate(0, 0, -DefaultTransactionExpirationDays)
	if err := tcm.unconfirmedTxRepo.DeleteExpiredTransactions(ctx, before); err != nil {
		logx.Errorf("Failed to cleanup expired transactions: %v", err)
	} else {
		logx.Info("✅ Cleaned up expired unconfirmed transactions")
	}

	// 清理Kafka重试队列中过期的成功消息
	if tcm.kafkaRetryQueue != nil {
		if err := tcm.kafkaRetryQueue.CleanupExpiredMessages(DefaultTransactionExpirationDays); err != nil {
			logx.Errorf("Failed to cleanup expired Kafka messages: %v", err)
		}
	}

	// 重试失败的交易
	tcm.retryFailedTransactions(ctx)
}

// retryFailedTransactions 重试失败的交易
func (tcm *TransactionConfirmManager) retryFailedTransactions(ctx context.Context) {
	transactions, err := tcm.unconfirmedTxRepo.GetRetryableTransactions(ctx)
	if err != nil {
		logx.Errorf("Failed to get retryable transactions: %v", err)
		return
	}

	if len(transactions) == 0 {
		return
	}

	logx.Infof("🔄 Retrying %d failed transactions", len(transactions))

	for _, tx := range transactions {
		// 增加重试次数
		if err := tcm.unconfirmedTxRepo.IncrementRetry(ctx, uint64(tx.ID)); err != nil {
			logx.Errorf("Failed to increment retry count for transaction %s: %v", tx.TxHash, err)
			continue
		}

		logx.Infof("🔄 Retrying transaction %s (retry count: %d)", tx.TxHash, tx.RetryCount+1)
	}
}
