package repository

import (
	"context"

	"internalwallet/services/admin/rpc/internal/model"

	"gorm.io/gorm"
)

type UserTransactionRecordRepository interface {
	WithTx(tx *gorm.DB) UserTransactionRecordRepository
	GetDB() *gorm.DB

	// 创建交易记录
	CreateRecord(ctx context.Context, tx *model.UserTransactionRecordModel) error

	// 更新已有记录状态（通过 idempotency_key 查找）
	// 返回更新的行数和错误
	UpdateStatusByIdempotencyKey(ctx context.Context, idempotencyKey, newStatus string, ledgerTxID *int64) (int64, error)
}

type userTransactionRecordRepo struct {
	db *gorm.DB
}

func NewUserTransactionRecordRepository(db *gorm.DB) UserTransactionRecordRepository {
	return &userTransactionRecordRepo{db: db}
}

func (r *userTransactionRecordRepo) WithTx(tx *gorm.DB) UserTransactionRecordRepository {
	return &userTransactionRecordRepo{db: tx}
}

func (r *userTransactionRecordRepo) GetDB() *gorm.DB {
	return r.db
}

func (r *userTransactionRecordRepo) CreateRecord(ctx context.Context, tx *model.UserTransactionRecordModel) error {
	return r.db.WithContext(ctx).Create(tx).Error
}

func (r *userTransactionRecordRepo) UpdateStatusByIdempotencyKey(ctx context.Context, idempotencyKey, newStatus string, ledgerTxID *int64) (int64, error) {
	updates := map[string]interface{}{
		"status": newStatus,
	}
	if ledgerTxID != nil {
		updates["settle_ledger_tx_id"] = *ledgerTxID
	}

	result := r.db.WithContext(ctx).
		Model(&model.UserTransactionRecordModel{}).
		Where("idempotency_key = ?", idempotencyKey).
		Updates(updates)

	return result.RowsAffected, result.Error
}
