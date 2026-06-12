package repository

import (
	"context"
	"time"

	"internalwallet/services/admin/rpc/internal/model"

	"gorm.io/gorm"
)

type CurrencyWithdrawOrderEventListFilter struct {
	WithdrawOrderID int64
	EventTypes      []string
	ActorTypes      []string
	DateFrom        *time.Time
	DateTo          *time.Time
}

type CurrencyWithdrawOrderEventRepository interface {
	WithTx(tx *gorm.DB) CurrencyWithdrawOrderEventRepository
	GetDB() *gorm.DB

	Create(ctx context.Context, m *model.CurrencyWithdrawOrderEventModel) error
	CreateMultiple(ctx context.Context, items []*model.CurrencyWithdrawOrderEventModel) error
	List(ctx context.Context, page, pageSize int32, f CurrencyWithdrawOrderEventListFilter) ([]*model.CurrencyWithdrawOrderEventModel, int64, error)
	FindByOrderAndEventType(ctx context.Context, withdrawOrderID int64, eventType string) ([]*model.CurrencyWithdrawOrderEventModel, error)
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

func (r *currencyWithdrawOrderEventRepo) List(ctx context.Context, page, pageSize int32, f CurrencyWithdrawOrderEventListFilter) ([]*model.CurrencyWithdrawOrderEventModel, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	query := r.db.WithContext(ctx).Model(&model.CurrencyWithdrawOrderEventModel{})
	if f.WithdrawOrderID > 0 {
		query = query.Where("withdraw_order_id = ?", f.WithdrawOrderID)
	}
	if len(f.EventTypes) > 0 {
		query = query.Where("event_type IN ?", f.EventTypes)
	}
	if len(f.ActorTypes) > 0 {
		query = query.Where("actor_type IN ?", f.ActorTypes)
	}
	if f.DateFrom != nil && !f.DateFrom.IsZero() {
		query = query.Where("created_at >= ?", *f.DateFrom)
	}
	if f.DateTo != nil && !f.DateTo.IsZero() {
		query = query.Where("created_at <= ?", *f.DateTo)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []*model.CurrencyWithdrawOrderEventModel
	offset := int((page - 1) * pageSize)
	err := query.Order("created_at DESC, id DESC").Offset(offset).Limit(int(pageSize)).Find(&items).Error
	return items, total, err
}

func (r *currencyWithdrawOrderEventRepo) FindByOrderAndEventType(ctx context.Context, withdrawOrderID int64, eventType string) ([]*model.CurrencyWithdrawOrderEventModel, error) {
	var items []*model.CurrencyWithdrawOrderEventModel
	err := r.db.WithContext(ctx).
		Where("withdraw_order_id = ? AND event_type = ?", withdrawOrderID, eventType).
		Order("created_at DESC, id DESC").
		Find(&items).Error
	return items, err
}
