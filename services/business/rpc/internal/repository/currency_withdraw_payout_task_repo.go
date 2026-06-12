package repository

import (
	"context"
	"fmt"
	"time"

	"internalwallet/services/business/rpc/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CurrencyWithdrawPayoutTaskRepository interface {
	WithTx(tx *gorm.DB) CurrencyWithdrawPayoutTaskRepository
	GetDB() *gorm.DB

	CreateIfNotExists(ctx context.Context, m *model.CurrencyWithdrawPayoutTaskModel) error
	FindByWithdrawOrderID(ctx context.Context, withdrawOrderID int64) (*model.CurrencyWithdrawPayoutTaskModel, error)
	UpdateFields(ctx context.Context, withdrawOrderID int64, fields map[string]interface{}) error

	ListDue(ctx context.Context, states []string, now time.Time, limit int) ([]*model.CurrencyWithdrawPayoutTaskModel, error)
	AcquireLock(ctx context.Context, withdrawOrderID int64, owner string, lockUntil time.Time, now time.Time) (bool, error)
	ReleaseLock(ctx context.Context, withdrawOrderID int64, owner string) error
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
	err := r.db.WithContext(ctx).
		Where("withdraw_order_id = ?", withdrawOrderID).
		First(&m).Error
	if err != nil {
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

func (r *currencyWithdrawPayoutTaskRepo) ListDue(ctx context.Context, states []string, now time.Time, limit int) ([]*model.CurrencyWithdrawPayoutTaskModel, error) {
	if limit <= 0 {
		limit = 20
	}
	var items []*model.CurrencyWithdrawPayoutTaskModel
	q := r.db.WithContext(ctx).
		Where("next_retry_time <= ? AND (lock_until IS NULL OR lock_until <= ?)", now, now).
		Order("next_retry_time ASC").
		Limit(limit)
	if len(states) > 0 {
		q = q.Where("state IN ?", states)
	}
	if err := q.Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *currencyWithdrawPayoutTaskRepo) AcquireLock(ctx context.Context, withdrawOrderID int64, owner string, lockUntil time.Time, now time.Time) (bool, error) {
	if withdrawOrderID <= 0 {
		return false, fmt.Errorf("invalid withdrawOrderID")
	}
	if owner == "" {
		return false, fmt.Errorf("invalid owner")
	}
	res := r.db.WithContext(ctx).
		Model(&model.CurrencyWithdrawPayoutTaskModel{}).
		Where("withdraw_order_id = ? AND (lock_until IS NULL OR lock_until <= ?)", withdrawOrderID, now).
		Updates(map[string]interface{}{
			"lock_owner": owner,
			"lock_until": lockUntil,
		})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}

func (r *currencyWithdrawPayoutTaskRepo) ReleaseLock(ctx context.Context, withdrawOrderID int64, owner string) error {
	if withdrawOrderID <= 0 || owner == "" {
		return nil
	}
	return r.db.WithContext(ctx).
		Model(&model.CurrencyWithdrawPayoutTaskModel{}).
		Where("withdraw_order_id = ? AND lock_owner = ?", withdrawOrderID, owner).
		Updates(map[string]interface{}{
			"lock_owner": nil,
			"lock_until": nil,
		}).Error
}
