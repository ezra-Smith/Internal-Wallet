package svc

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/services/chainsync/rpc/internal/model"
	"internalwallet/services/chainsync/rpc/internal/repository"
)

// KafkaRetryQueue Kafka重试队列管理器
type KafkaRetryQueue struct {
	failedMessageRepo repository.KafkaFailedMessageRepository
	kafkaProducer     KafkaProducerInterface

	// 配置
	maxRetries        int32
	initialDelay      time.Duration
	maxDelay          time.Duration
	backoffMultiplier float64
	checkInterval     time.Duration
	batchSize         int

	// 控制
	stopCh chan struct{}
	wg     sync.WaitGroup

	// 统计
	stats *KafkaRetryStats
	mu    sync.RWMutex
}

// KafkaRetryStats Kafka重试统计
type KafkaRetryStats struct {
	TotalQueued     uint64    `json:"total_queued"`
	TotalRetried    uint64    `json:"total_retried"`
	TotalSucceeded  uint64    `json:"total_succeeded"`
	TotalFailed     uint64    `json:"total_failed"`
	PendingCount    int64     `json:"pending_count"`
	LastRetryTime   time.Time `json:"last_retry_time"`
	LastSuccessTime time.Time `json:"last_success_time"`
}

// NewKafkaRetryQueue 创建Kafka重试队列
func NewKafkaRetryQueue(
	failedMessageRepo repository.KafkaFailedMessageRepository,
	kafkaProducer KafkaProducerInterface,
) *KafkaRetryQueue {
	return &KafkaRetryQueue{
		failedMessageRepo: failedMessageRepo,
		kafkaProducer:     kafkaProducer,
		maxRetries:        DefaultKafkaRetryMaxAttempts,
		initialDelay:      DefaultKafkaRetryInitialDelay,
		maxDelay:          DefaultKafkaRetryMaxDelay,
		backoffMultiplier: DefaultKafkaRetryBackoffMultiplier,
		checkInterval:     DefaultKafkaRetryQueueCheckInterval,
		batchSize:         DefaultKafkaRetryQueueBatchSize,
		stopCh:            make(chan struct{}),
		stats:             &KafkaRetryStats{},
	}
}

// Start 启动重试队列
func (q *KafkaRetryQueue) Start() {
	logx.Info("🚀 Starting Kafka retry queue...")

	q.wg.Add(1)
	go q.retryLoop()

	logx.Info("✅ Kafka retry queue started")
}

// Stop 停止重试队列
func (q *KafkaRetryQueue) Stop() {
	logx.Info("Stopping Kafka retry queue...")

	close(q.stopCh)
	q.wg.Wait()

	// 输出最终统计
	q.printStats()

	logx.Info("✅ Kafka retry queue stopped")
}

// retryLoop 重试循环
func (q *KafkaRetryQueue) retryLoop() {
	defer q.wg.Done()

	ticker := time.NewTicker(q.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-q.stopCh:
			logx.Info("Kafka retry loop stopped")
			return

		case <-ticker.C:
			q.processRetryBatch()
		}
	}
}

// processRetryBatch 处理重试批次
func (q *KafkaRetryQueue) processRetryBatch() {
	if q.failedMessageRepo == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), DefaultTransactionConfirmContextTimeout*time.Second)
	defer cancel()

	// 获取可重试的消息
	messages, err := q.failedMessageRepo.GetRetryableMessages(ctx, q.batchSize)
	if err != nil {
		logx.Errorf("Failed to get retryable messages: %v", err)
		return
	}

	if len(messages) == 0 {
		return
	}

	logx.Infof("🔄 Processing %d Kafka retry messages", len(messages))

	successCount := 0
	failCount := 0

	for _, msg := range messages {
		if q.retryMessage(ctx, msg) {
			successCount++
		} else {
			failCount++
		}
	}

	q.mu.Lock()
	q.stats.LastRetryTime = time.Now()
	q.mu.Unlock()

	logx.Infof("✅ Kafka retry batch completed: %d succeeded, %d failed", successCount, failCount)
}

// retryMessage 重试单条消息
func (q *KafkaRetryQueue) retryMessage(ctx context.Context, msg *model.KafkaFailedMessage) bool {
	if q.kafkaProducer == nil {
		logx.Error("Kafka producer is nil, cannot retry message")
		return false
	}

	// 尝试发送消息
	messageID, err := q.kafkaProducer.SendMessage(msg.Topic, msg.Key, []byte(msg.Message))
	if err != nil {
		// 发送失败，更新重试信息
		q.handleRetryFailure(ctx, msg, err)
		return false
	}

	// 发送成功，标记为成功
	q.handleRetrySuccess(ctx, msg, messageID)
	return true
}

// handleRetrySuccess 处理重试成功
func (q *KafkaRetryQueue) handleRetrySuccess(ctx context.Context, msg *model.KafkaFailedMessage, messageID string) {
	if err := q.failedMessageRepo.MarkAsSuccess(ctx, uint64(msg.ID), messageID); err != nil {
		logx.Errorf("Failed to mark message %d as success: %v", msg.ID, err)
	} else {
		logx.Infof("✅ Kafka message %d sent successfully after retry, messageID: %s", msg.ID, messageID)
	}

	q.mu.Lock()
	q.stats.TotalSucceeded++
	q.stats.LastSuccessTime = time.Now()
	q.mu.Unlock()
}

// handleRetryFailure 处理重试失败
func (q *KafkaRetryQueue) handleRetryFailure(ctx context.Context, msg *model.KafkaFailedMessage, err error) {
	q.mu.Lock()
	q.stats.TotalRetried++
	q.mu.Unlock()

	// 计算下次重试时间
	msg.IncrementRetry(q.initialDelay, q.maxDelay, q.backoffMultiplier)

	if msg.RetryCount >= msg.MaxRetries {
		// 超过最大重试次数，标记为失败
		if markErr := q.failedMessageRepo.MarkAsFailed(ctx, uint64(msg.ID), err.Error()); markErr != nil {
			logx.Errorf("Failed to mark message %d as failed: %v", msg.ID, markErr)
		} else {
			logx.Errorf("❌ Kafka message %d failed after %d retries: %v", msg.ID, msg.RetryCount, err)
		}

		q.mu.Lock()
		q.stats.TotalFailed++
		q.mu.Unlock()
	} else {
		// 更新重试信息
		if updateErr := q.failedMessageRepo.IncrementRetry(ctx, uint64(msg.ID), *msg.NextRetryAt, err.Error()); updateErr != nil {
			logx.Errorf("Failed to update retry info for message %d: %v", msg.ID, updateErr)
		} else {
			logx.Infof("⚠️ Kafka message %d retry %d/%d failed, next retry at %v: %v",
				msg.ID, msg.RetryCount, msg.MaxRetries, msg.NextRetryAt, err)
		}
	}
}

// EnqueueFailedMessage 将失败消息加入重试队列
func (q *KafkaRetryQueue) EnqueueFailedMessage(
	ctx context.Context,
	topic string,
	key string,
	message interface{},
	messageType string,
	txHash string,
	chain string,
	sourceModule string,
	priority int32,
) error {
	if q.failedMessageRepo == nil {
		return nil // 如果没有仓储，直接返回(降级处理)
	}

	// 序列化消息
	var messageStr string
	switch v := message.(type) {
	case []byte:
		messageStr = string(v)
	case string:
		messageStr = v
	default:
		messageBytes, err := json.Marshal(message)
		if err != nil {
			logx.Errorf("Failed to marshal message for retry queue: %v", err)
			return err
		}
		messageStr = string(messageBytes)
	}

	// 创建失败消息记录
	now := time.Now()
	failedMsg := &model.KafkaFailedMessage{
		Topic:        topic,
		Key:          key,
		Message:      messageStr,
		MessageType:  messageType,
		TxHash:       txHash,
		Chain:        chain,
		Status:       KafkaRetryStatusPending,
		RetryCount:   0,
		MaxRetries:   q.maxRetries,
		NextRetryAt:  &now,
		Priority:     priority,
		SourceModule: sourceModule,
	}

	if err := q.failedMessageRepo.Create(ctx, failedMsg); err != nil {
		logx.Errorf("Failed to enqueue Kafka message for retry: %v", err)
		return err
	}

	q.mu.Lock()
	q.stats.TotalQueued++
	q.mu.Unlock()

	logx.Infof("📥 Enqueued Kafka message for retry: topic=%s, key=%s, type=%s", topic, key, messageType)
	return nil
}

// GetStats 获取统计信息
func (q *KafkaRetryQueue) GetStats() *KafkaRetryStats {
	q.mu.RLock()
	defer q.mu.RUnlock()

	// 获取待处理数量
	var pendingCount int64
	if q.failedMessageRepo != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if count, err := q.failedMessageRepo.GetPendingCount(ctx); err == nil {
			pendingCount = count
		}
		cancel()
	}

	return &KafkaRetryStats{
		TotalQueued:     q.stats.TotalQueued,
		TotalRetried:    q.stats.TotalRetried,
		TotalSucceeded:  q.stats.TotalSucceeded,
		TotalFailed:     q.stats.TotalFailed,
		PendingCount:    pendingCount,
		LastRetryTime:   q.stats.LastRetryTime,
		LastSuccessTime: q.stats.LastSuccessTime,
	}
}

// printStats 打印统计信息
func (q *KafkaRetryQueue) printStats() {
	stats := q.GetStats()
	logx.Infof("📊 Kafka Retry Queue Statistics:")
	logx.Infof("   Total Queued:    %d", stats.TotalQueued)
	logx.Infof("   Total Retried:   %d", stats.TotalRetried)
	logx.Infof("   Total Succeeded: %d", stats.TotalSucceeded)
	logx.Infof("   Total Failed:    %d", stats.TotalFailed)
	logx.Infof("   Pending Count:   %d", stats.PendingCount)
}

// CleanupExpiredMessages 清理过期的成功消息
func (q *KafkaRetryQueue) CleanupExpiredMessages(retentionDays int) error {
	if q.failedMessageRepo == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	before := time.Now().AddDate(0, 0, -retentionDays)
	if err := q.failedMessageRepo.DeleteExpiredMessages(ctx, before); err != nil {
		logx.Errorf("Failed to cleanup expired Kafka messages: %v", err)
		return err
	}

	logx.Infof("✅ Cleaned up expired Kafka messages older than %d days", retentionDays)
	return nil
}
