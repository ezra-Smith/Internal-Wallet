package repository

import (
	"context"
	"errors"
	commonRepo "internalwallet/common/repository"
	"time"

	"gorm.io/gorm"

	"internalwallet/services/chainsync/rpc/internal/model"
)

// UnconfirmedTransactionRepository 未确认交易仓储接口
type UnconfirmedTransactionRepository interface {
	commonRepo.BaseRepository[model.UnconfirmedTransaction]

	// 根据交易哈希获取交易
	GetByTxHash(ctx context.Context, txHash string) (*model.UnconfirmedTransaction, error)

	// 根据交易哈希、监控地址、来源和日志索引获取交易（用于支持多地址/多来源/多Transfer场景）
	GetByTxHashAndAddressSource(ctx context.Context, txHash string, monitoredAddress string, source string, logIndex int32) (*model.UnconfirmedTransaction, error)

	// 根据交易哈希和日志索引获取交易（用于去重检查，不考虑监控地址）
	GetByTxHashAndLogIndex(ctx context.Context, txHash string, logIndex int32) (*model.UnconfirmedTransaction, error)

	// 获取待确认的交易列表（分页）
	GetPendingTransactions(ctx context.Context, offset, limit int) ([]*model.UnconfirmedTransaction, error)

	// 获取需要确认的交易（当前区块大于交易区块+确认数）
	GetTransactionsToConfirm(ctx context.Context, chain string, currentBlock uint64, confirmations int32) ([]*model.UnconfirmedTransaction, error)

	// 获取监控地址的未确认交易
	GetByMonitoredAddress(ctx context.Context, chain, address string) ([]*model.UnconfirmedTransaction, error)

	// 更新交易状态
	UpdateStatus(ctx context.Context, id uint64, status uint8, kafkaMessageID, kafkaTopic string) error

	// 更新确认数
	UpdateConfirmations(ctx context.Context, id uint64, confirmations int32) error

	// 更新链上执行状态（exec_status）与节流信息
	UpdateExecStatus(ctx context.Context, id uint64, execStatus model.ExecStatus, checkedAt time.Time, nextCheckAt *time.Time, execErrMsg string) error

	// 标记为链上执行失败（终止），避免继续下发 Kafka/重试
	MarkAsExecFailedTerminal(ctx context.Context, id uint64, execErrMsg string) error

	// 标记为已发送Kafka
	MarkAsSent(ctx context.Context, id uint64, kafkaMessageID, kafkaTopic string) error

	// 标记为确认失败
	MarkAsFailed(ctx context.Context, id uint64, errorMessage string) error

	// 增加重试次数
	IncrementRetry(ctx context.Context, id uint64) error

	// 删除过期的未确认交易（超过7天）
	DeleteExpiredTransactions(ctx context.Context, before time.Time) error

	// 批量更新确认数
	BatchUpdateConfirmations(ctx context.Context, updates map[uint64]int32) error

	// 获取需要重试的交易
	GetRetryableTransactions(ctx context.Context) ([]*model.UnconfirmedTransaction, error)
}

// unconfirmedTransactionRepository 实现
type unconfirmedTransactionRepository struct {
	commonRepo.BaseRepository[model.UnconfirmedTransaction]
}

// NewUnconfirmedTransactionRepository 创建未确认交易仓储
func NewUnconfirmedTransactionRepository(db *gorm.DB) UnconfirmedTransactionRepository {
	return &unconfirmedTransactionRepository{
		BaseRepository: commonRepo.NewBaseRepository[model.UnconfirmedTransaction](db),
	}
}

// GetByTxHash 根据交易哈希获取交易
func (r *unconfirmedTransactionRepository) GetByTxHash(ctx context.Context, txHash string) (*model.UnconfirmedTransaction, error) {
	var transaction model.UnconfirmedTransaction
	err := r.GetDB().WithContext(ctx).Where("tx_hash = ?", txHash).First(&transaction).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &transaction, nil
}

// GetByTxHashAndAddressSource 根据交易哈希、监控地址、来源和日志索引获取交易
// 用于支持 Swap、多监控地址、多来源归属、授权转账等场景
func (r *unconfirmedTransactionRepository) GetByTxHashAndAddressSource(
	ctx context.Context,
	txHash string,
	monitoredAddress string,
	source string,
	logIndex int32,
) (*model.UnconfirmedTransaction, error) {
	var transaction model.UnconfirmedTransaction
	err := r.GetDB().WithContext(ctx).
		Where("tx_hash = ? AND monitored_address = ? AND source = ? AND log_index = ?",
			txHash, monitoredAddress, source, logIndex).
		First(&transaction).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &transaction, nil
}

// GetByTxHashAndLogIndex 根据交易哈希和日志索引获取交易（不考虑监控地址）
// 用于去重检查：同一交易的同一个 Transfer 事件只保存一次
func (r *unconfirmedTransactionRepository) GetByTxHashAndLogIndex(
	ctx context.Context,
	txHash string,
	logIndex int32,
) (*model.UnconfirmedTransaction, error) {
	var transaction model.UnconfirmedTransaction
	err := r.GetDB().WithContext(ctx).
		Where("tx_hash = ? AND log_index = ?", txHash, logIndex).
		First(&transaction).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &transaction, nil
}

// GetPendingTransactions 获取待确认的交易列表（分页）
func (r *unconfirmedTransactionRepository) GetPendingTransactions(ctx context.Context, offset, limit int) ([]*model.UnconfirmedTransaction, error) {
	var transactions []*model.UnconfirmedTransaction
	err := r.GetDB().WithContext(ctx).
		Where("status = ?", 0).
		Order("created_at ASC").
		Offset(offset).
		Limit(limit).
		Find(&transactions).Error
	return transactions, err
}

// GetTransactionsToConfirm 获取需要确认的交易
func (r *unconfirmedTransactionRepository) GetTransactionsToConfirm(ctx context.Context, chain string, currentBlock uint64, confirmations int32) ([]*model.UnconfirmedTransaction, error) {
	var transactions []*model.UnconfirmedTransaction

	// 计算最小区块号（当前区块 - 需要的确认数）
	minBlock, ok := minBlockForConfirmations(currentBlock, confirmations)
	if !ok {
		return []*model.UnconfirmedTransaction{}, nil
	}

	err := r.GetDB().WithContext(ctx).
		Select("id, tx_hash, chain, block_number, block_hash, from_address, to_address, monitored_address, source, direction, counterparty_address, monitored_is_internal, counterparty_is_internal, counterparty_source_bits, value, gas_price, gas_used, gas_fee, energy_used, bandwidth_used, transaction_index, block_timestamp, log_index, transaction_type, token_address, token_name, token_symbol, token_decimals, token_amount, exec_status, exec_checked_at, exec_next_check_at, exec_error_message, status, confirmations, required_confirmations, message_id, message_topic, sent_at, error_message, retry_count, max_retries, created_at, updated_at, deleted_at").
		Where("chain = ? AND status = 0 AND block_number <= ?", chain, minBlock).
		Order("block_number ASC").
		Limit(1000). // 添加限制，防止一次性查询过多数据
		Find(&transactions).Error

	return transactions, err
}

// GetByMonitoredAddress 获取监控地址的未确认交易
func (r *unconfirmedTransactionRepository) GetByMonitoredAddress(ctx context.Context, chain, address string) ([]*model.UnconfirmedTransaction, error) {
	var transactions []*model.UnconfirmedTransaction
	err := r.GetDB().WithContext(ctx).
		Where("chain = ? AND (monitored_address = ? OR from_address = ? OR to_address = ?)",
			chain, address, address, address).
		Order("created_at DESC").
		Limit(100).
		Find(&transactions).Error
	return transactions, err
}

// UpdateStatus 更新交易状态
func (r *unconfirmedTransactionRepository) UpdateStatus(ctx context.Context, id uint64, status uint8, kafkaMessageID, kafkaTopic string) error {
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now().Local(),
	}

	if kafkaMessageID != "" {
		updates["message_id"] = kafkaMessageID
	}
	if kafkaTopic != "" {
		updates["message_topic"] = kafkaTopic
	}

	return r.GetDB().WithContext(ctx).
		Model(&model.UnconfirmedTransaction{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// UpdateConfirmations 更新确认数
func (r *unconfirmedTransactionRepository) UpdateConfirmations(ctx context.Context, id uint64, confirmations int32) error {
	return r.GetDB().WithContext(ctx).
		Model(&model.UnconfirmedTransaction{}).
		Where("id = ?", id).
		Update("confirmations", confirmations).Error
}

func (r *unconfirmedTransactionRepository) UpdateExecStatus(ctx context.Context, id uint64, execStatus model.ExecStatus, checkedAt time.Time, nextCheckAt *time.Time, execErrMsg string) error {
	updates := map[string]interface{}{
		"exec_status":        execStatus,
		"exec_checked_at":    &checkedAt,
		"exec_error_message": execErrMsg,
		"updated_at":         time.Now().Local(),
	}
	if nextCheckAt != nil {
		updates["exec_next_check_at"] = nextCheckAt
	} else {
		updates["exec_next_check_at"] = nil
	}
	return r.GetDB().WithContext(ctx).
		Model(&model.UnconfirmedTransaction{}).
		Where("id = ?", id).
		Updates(updates).Error
}

func (r *unconfirmedTransactionRepository) MarkAsExecFailedTerminal(ctx context.Context, id uint64, execErrMsg string) error {
	now := time.Now().Local()
	// status=2 marks as failed/terminal; MaxRetries=0 prevents retry loops that use status=2.
	return r.GetDB().WithContext(ctx).
		Model(&model.UnconfirmedTransaction{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"exec_status":        model.ExecStatusFailed,
			"exec_checked_at":    &now,
			"exec_next_check_at": nil,
			"exec_error_message": execErrMsg,
			"status":             2,
			"error_message":      execErrMsg,
			"max_retries":        int32(0),
			"updated_at":         now,
		}).Error
}

// MarkAsSent 标记为已发送Kafka
func (r *unconfirmedTransactionRepository) MarkAsSent(ctx context.Context, id uint64, kafkaMessageID, kafkaTopic string) error {
	now := time.Now().Local()
	return r.GetDB().WithContext(ctx).
		Model(&model.UnconfirmedTransaction{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":        1,
			"message_id":    kafkaMessageID,
			"message_topic": kafkaTopic,
			"sent_at":       &now,
			//"confirmed_at":     &now,
			"updated_at": now,
		}).Error
}

// MarkAsFailed 标记为确认失败
func (r *unconfirmedTransactionRepository) MarkAsFailed(ctx context.Context, id uint64, errorMessage string) error {
	return r.GetDB().WithContext(ctx).
		Model(&model.UnconfirmedTransaction{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":        2,
			"error_message": errorMessage,
			"updated_at":    time.Now().Local(),
		}).Error
}

// IncrementRetry 增加重试次数
func (r *unconfirmedTransactionRepository) IncrementRetry(ctx context.Context, id uint64) error {
	return r.GetDB().WithContext(ctx).
		Model(&model.UnconfirmedTransaction{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"retry_count":   gorm.Expr("retry_count + 1"),
			"status":        0,
			"error_message": "",
			"updated_at":    time.Now().Local(),
		}).Error
}

// DeleteExpiredTransactions 删除过期的未确认交易（超过7天）
func (r *unconfirmedTransactionRepository) DeleteExpiredTransactions(ctx context.Context, before time.Time) error {
	return r.GetDB().WithContext(ctx).
		Where("status != 1 AND created_at < ?", before).
		Delete(&model.UnconfirmedTransaction{}).Error
}

// BatchUpdateConfirmations 批量更新确认数
func (r *unconfirmedTransactionRepository) BatchUpdateConfirmations(ctx context.Context, updates map[uint64]int32) error {
	return r.GetDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for id, confirmations := range updates {
			if err := tx.Model(&model.UnconfirmedTransaction{}).
				Where("id = ?", id).
				Update("confirmations", confirmations).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// GetRetryableTransactions 获取需要重试的交易
func (r *unconfirmedTransactionRepository) GetRetryableTransactions(ctx context.Context) ([]*model.UnconfirmedTransaction, error) {
	var transactions []*model.UnconfirmedTransaction
	err := r.GetDB().WithContext(ctx).
		Where("status = 2 AND retry_count < max_retries").
		Order("updated_at ASC").
		Find(&transactions).Error
	return transactions, err
}
