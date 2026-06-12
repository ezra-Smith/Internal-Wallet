package repository

import (
	"context"

	"internalwallet/services/business/rpc/internal/model"

	"gorm.io/gorm"
)

type CurrencyWithdrawOrderEventRepository interface {
	WithTx(tx *gorm.DB) CurrencyWithdrawOrderEventRepository
	GetDB() *gorm.DB

	Create(ctx context.Context, m *model.CurrencyWithdrawOrderEventModel) error
	CreateMultiple(ctx context.Context, items []*model.CurrencyWithdrawOrderEventModel) error
}

type currencyWithdrawOrderEventRepo struct{ db *gorm.DB }

func NewCurrencyWithdrawOrderEventRepository(db *gorm.DB) CurrencyWithdrawOrderEventRepository {
	return &currencyWithdrawOrderEventRepo{db: db}
}

func (r *currencyWithdrawOrderEventRepo) WithTx(tx *gorm.DB) CurrencyWithdrawOrderEventRepository {
	return &currencyWithdrawOrderEventRepo{db: tx}
}

func (r *currencyWithdrawOrderEventRepo) GetDB() *gorm.DB { return r.db }

func (r *currencyWithdrawOrderEventRepo) Create(ctx context.Context, m *model.CurrencyWithdrawOrderEventModel) error {
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *currencyWithdrawOrderEventRepo) CreateMultiple(ctx context.Context, items []*model.CurrencyWithdrawOrderEventModel) error {
	if len(items) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).CreateInBatches(items, 100).Error
}
