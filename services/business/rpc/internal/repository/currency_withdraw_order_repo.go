package repository

import (
	"context"
	"errors"
	"fmt"

	commonRepo "internalwallet/common/repository"
	"internalwallet/services/business/rpc/internal/model"

	"gorm.io/gorm"
)

type CurrencyWithdrawOrderRepository interface {
	commonRepo.BaseRepository[model.CurrencyWithdrawOrderModel]
	CreateOrder(ctx context.Context, order *model.CurrencyWithdrawOrderModel) error
	FindByID(ctx context.Context, id int64) (*model.CurrencyWithdrawOrderModel, error)
	FindByUserIDAndID(ctx context.Context, userID, id int64) (*model.CurrencyWithdrawOrderModel, error)
	UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error
}

type currencyWithdrawOrderRepo struct {
	commonRepo.BaseRepository[model.CurrencyWithdrawOrderModel]
}

func NewCurrencyWithdrawOrderRepository(db *gorm.DB) CurrencyWithdrawOrderRepository {
	return &currencyWithdrawOrderRepo{
		BaseRepository: commonRepo.NewBaseRepository[model.CurrencyWithdrawOrderModel](db),
	}
}

func (r *currencyWithdrawOrderRepo) CreateOrder(ctx context.Context, order *model.CurrencyWithdrawOrderModel) error {
	if order == nil {
		return fmt.Errorf("order is nil")
	}
	return r.GetDB().WithContext(ctx).Create(order).Error
}

func (r *currencyWithdrawOrderRepo) FindByID(ctx context.Context, id int64) (*model.CurrencyWithdrawOrderModel, error) {
	if id <= 0 {
		return nil, fmt.Errorf("invalid id")
	}
	var m model.CurrencyWithdrawOrderModel
	err := r.GetDB().WithContext(ctx).First(&m, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, gorm.ErrRecordNotFound
	}
	return &m, err
}

func (r *currencyWithdrawOrderRepo) FindByUserIDAndID(ctx context.Context, userID, id int64) (*model.CurrencyWithdrawOrderModel, error) {
	if userID <= 0 || id <= 0 {
		return nil, fmt.Errorf("invalid params")
	}
	var m model.CurrencyWithdrawOrderModel
	err := r.GetDB().WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, gorm.ErrRecordNotFound
	}
	return &m, err
}

func (r *currencyWithdrawOrderRepo) UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error {
	if id <= 0 {
		return fmt.Errorf("invalid id")
	}
	if len(fields) == 0 {
		return nil
	}
	res := r.GetDB().WithContext(ctx).
		Model(&model.CurrencyWithdrawOrderModel{}).
		Where("id = ?", id).
		Updates(fields)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
