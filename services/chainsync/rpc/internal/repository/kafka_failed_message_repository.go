package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"internalwallet/services/chainsync/rpc/internal/model"
)

// KafkaFailedMessageRepository Kafka失败消息仓储接口
type KafkaFailedMessageRepository interface {
	// Create 创建失败消息记录
	Create(ctx context.Context, message *model.KafkaFailedMessage) error

	// GetByID 根据ID获取消息
	GetByID(ctx context.Context, id uint64) (*model.KafkaFailedMessage, error)

	// GetPendingMessages 获取待重试的消息列表
	GetPendingMessages(ctx context.Context, limit int) ([]*model.KafkaFailedMessage, error)

	// GetRetryableMessages 获取可重试的消息(已到重试时间)
	GetRetryableMessages(ctx context.Context, limit int) ([]*model.KafkaFailedMessage, error)

	// GetByTxHash 根据交易哈希获取消息
	GetByTxHash(ctx context.Context, txHash string) ([]*model.KafkaFailedMessage, error)

	// UpdateStatus 更新消息状态
	UpdateStatus(ctx context.Context, id uint64, status uint8) error

	// MarkAsSuccess 标记为成功
	MarkAsSuccess(ctx context.Context, id uint64, messageID string) error

	// MarkAsFailed 标记为失败
	MarkAsFailed(ctx context.Context, id uint64, errorMessage string) error

	// IncrementRetry 增加重试次数并更新下次重试时间
	IncrementRetry(ctx context.Context, id uint64, nextRetryAt time.Time, lastError string) error

	// DeleteExpiredMessages 删除过期的成功消息
	DeleteExpiredMessages(ctx context.Context, before time.Time) error

	// GetPendingCount 获取待重试消息数量
	GetPendingCount(ctx context.Context) (int64, error)

	// BatchCreate 批量创建失败消息
	BatchCreate(ctx context.Context, messages []*model.KafkaFailedMessage) error
}

// kafkaFailedMessageRepository 实现
type kafkaFailedMessageRepository struct {
	db *gorm.DB
}

// NewKafkaFailedMessageRepository 创建仓储实例
func NewKafkaFailedMessageRepository(db *gorm.DB) KafkaFailedMessageRepository {
	return &kafkaFailedMessageRepository{
		db: db,
	}
}

// Create 创建失败消息记录
func (r *kafkaFailedMessageRepository) Create(ctx context.Context, message *model.KafkaFailedMessage) error {
	return r.db.WithContext(ctx).Create(message).Error
}

// GetByID 根据ID获取消息
func (r *kafkaFailedMessageRepository) GetByID(ctx context.Context, id uint64) (*model.KafkaFailedMessage, error) {
	var message model.KafkaFailedMessage
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&message).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &message, nil
}

// GetPendingMessages 获取待重试的消息列表
func (r *kafkaFailedMessageRepository) GetPendingMessages(ctx context.Context, limit int) ([]*model.KafkaFailedMessage, error) {
	var messages []*model.KafkaFailedMessage
	err := r.db.WithContext(ctx).
		Where("status = ?", 0).
		Order("priority DESC, created_at ASC").
		Limit(limit).
		Find(&messages).Error
	return messages, err
}

// GetRetryableMessages 获取可重试的消息(已到重试时间)
func (r *kafkaFailedMessageRepository) GetRetryableMessages(ctx context.Context, limit int) ([]*model.KafkaFailedMessage, error) {
	var messages []*model.KafkaFailedMessage
	now := time.Now().Local()
	err := r.db.WithContext(ctx).
		Where("status = ? AND retry_count < max_retries AND (next_retry_at IS NULL OR next_retry_at <= ?)", 0, now).
		Order("priority DESC, next_retry_at ASC").
		Limit(limit).
		Find(&messages).Error
	return messages, err
}

// GetByTxHash 根据交易哈希获取消息
func (r *kafkaFailedMessageRepository) GetByTxHash(ctx context.Context, txHash string) ([]*model.KafkaFailedMessage, error) {
	var messages []*model.KafkaFailedMessage
	err := r.db.WithContext(ctx).
		Where("tx_hash = ?", txHash).
		Order("created_at DESC").
		Find(&messages).Error
	return messages, err
}

// UpdateStatus 更新消息状态
func (r *kafkaFailedMessageRepository) UpdateStatus(ctx context.Context, id uint64, status uint8) error {
	return r.db.WithContext(ctx).
		Model(&model.KafkaFailedMessage{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// MarkAsSuccess 标记为成功
func (r *kafkaFailedMessageRepository) MarkAsSuccess(ctx context.Context, id uint64, messageID string) error {
	now := time.Now().Local()
	return r.db.WithContext(ctx).
		Model(&model.KafkaFailedMessage{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":     1,
			"success_at": &now,
			"message_id": messageID,
			"last_error": "",
			"updated_at": now,
		}).Error
}

// MarkAsFailed 标记为失败
func (r *kafkaFailedMessageRepository) MarkAsFailed(ctx context.Context, id uint64, errorMessage string) error {
	return r.db.WithContext(ctx).
		Model(&model.KafkaFailedMessage{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":     2,
			"last_error": errorMessage,
			"updated_at": time.Now().Local(),
		}).Error
}

// IncrementRetry 增加重试次数并更新下次重试时间
func (r *kafkaFailedMessageRepository) IncrementRetry(ctx context.Context, id uint64, nextRetryAt time.Time, lastError string) error {
	now := time.Now().Local()
	return r.db.WithContext(ctx).
		Model(&model.KafkaFailedMessage{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"retry_count":   gorm.Expr("retry_count + 1"),
			"next_retry_at": &nextRetryAt,
			"last_retry_at": &now,
			"last_error":    lastError,
			"updated_at":    now,
		}).Error
}

// DeleteExpiredMessages 删除过期的成功消息
func (r *kafkaFailedMessageRepository) DeleteExpiredMessages(ctx context.Context, before time.Time) error {
	return r.db.WithContext(ctx).
		Where("status = 1 AND success_at < ?", before).
		Delete(&model.KafkaFailedMessage{}).Error
}

// GetPendingCount 获取待重试消息数量
func (r *kafkaFailedMessageRepository) GetPendingCount(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.KafkaFailedMessage{}).
		Where("status = ?", 0).
		Count(&count).Error
	return count, err
}

// BatchCreate 批量创建失败消息
func (r *kafkaFailedMessageRepository) BatchCreate(ctx context.Context, messages []*model.KafkaFailedMessage) error {
	if len(messages) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).CreateInBatches(messages, 100).Error
}
