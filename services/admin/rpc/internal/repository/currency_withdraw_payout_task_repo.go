package repository

import (
	"context"
	"fmt"

	"internalwallet/services/admin/rpc/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CurrencyWithdrawPayoutTaskRepository interface {
	WithTx(tx *gorm.DB) CurrencyWithdrawPayoutTaskRepository
	GetDB() *gorm.DB

	CreateIfNotExists(ctx context.Context, m *model.CurrencyWithdrawPayoutTaskModel) error
	FindByWithdrawOrderID(ctx context.Context, withdrawOrderID int64) (*model.CurrencyWithdrawPayoutTaskModel, error)
	UpdateFields(ctx context.Context, withdrawOrderID int64, fields map[string]interface{}) error
}

type currencyWithdrawPayoutTaskRepo struct{ db *gorm.DB }

func NewCurrencyWithdrawPayoutTaskRepository(db *gorm.DB) CurrencyWithdrawPayoutTaskRepository {
	return &currencyWithdrawPayoutTaskRepo{db: db}
}

func (r *currencyWithdrawPayoutTaskRepo) WithTx(tx *gorm.DB) CurrencyWithdrawPayoutTaskRepository {
	return &currencyWithdrawPayoutTaskRepo{db: tx}
}

func (r *currencyWithdrawPayoutTaskRepo) GetDB() *gorm.DB { return r.db }

func (r *currencyWithdrawPayoutTaskRepo) CreateIfNotExists(ctx context.Context, m *model.CurrencyWithdrawPayoutTaskModel) error {
	if m == nil {
		return fmt.Errorf("task is nil")
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(m).Error
}

func (r *currencyWithdrawPayoutTaskRepo) FindByWithdrawOrderID(ctx context.Context, withdrawOrderID int64) (*model.CurrencyWithdrawPayoutTaskModel, error) {
	if withdrawOrderID <= 0 {
		return nil, fmt.Errorf("invalid withdrawOrderID")
	}
	var m model.CurrencyWithdrawPayoutTaskModel
	if err := r.db.WithContext(ctx).
		Where("withdraw_order_id = ?", withdrawOrderID).
		First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *currencyWithdrawPayoutTaskRepo) UpdateFields(ctx context.Context, withdrawOrderID int64, fields map[string]interface{}) error {
	if withdrawOrderID <= 0 {
		return fmt.Errorf("invalid withdrawOrderID")
	}
	if len(fields) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Model(&model.CurrencyWithdrawPayoutTaskModel{}).
		Where("withdraw_order_id = ?", withdrawOrderID).
		Updates(fields).Error
}
