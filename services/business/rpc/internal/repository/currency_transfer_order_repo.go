package repository

import (
	"context"
	"strings"

	"internalwallet/services/business/rpc/internal/model"

	"gorm.io/gorm"
)

type CurrencyTransferOrderRepository interface {
	WithTx(tx *gorm.DB) CurrencyTransferOrderRepository
	GetDB() *gorm.DB

	Create(ctx context.Context, order *model.CurrencyTransferOrderModel) error
	GetByID(ctx context.Context, id int64) (*model.CurrencyTransferOrderModel, error)
	Update(ctx context.Context, order *model.CurrencyTransferOrderModel) error
	ListByUser(ctx context.Context, userID int64, assetCode, status, direction string, page, pageSize int) ([]*model.CurrencyTransferOrderModel, int64, error)
}

type currencyTransferOrderRepo struct {
	db *gorm.DB
}

func NewCurrencyTransferOrderRepository(db *gorm.DB) CurrencyTransferOrderRepository {
	return &currencyTransferOrderRepo{db: db}
}

func (r *currencyTransferOrderRepo) WithTx(tx *gorm.DB) CurrencyTransferOrderRepository {
	return &currencyTransferOrderRepo{db: tx}
}

func (r *currencyTransferOrderRepo) GetDB() *gorm.DB {
	return r.db
}

func (r *currencyTransferOrderRepo) Create(ctx context.Context, order *model.CurrencyTransferOrderModel) error {
	return r.db.WithContext(ctx).Create(order).Error
}

func (r *currencyTransferOrderRepo) GetByID(ctx context.Context, id int64) (*model.CurrencyTransferOrderModel, error) {
	var order model.CurrencyTransferOrderModel
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&order).Error
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func (r *currencyTransferOrderRepo) Update(ctx context.Context, order *model.CurrencyTransferOrderModel) error {
	return r.db.WithContext(ctx).Save(order).Error
}

func (r *currencyTransferOrderRepo) ListByUser(ctx context.Context, userID int64, assetCode, status, direction string, page, pageSize int) ([]*model.CurrencyTransferOrderModel, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.CurrencyTransferOrderModel{})

	// Filter by user (either from or to)
	direction = strings.ToLower(strings.TrimSpace(direction))
	switch direction {
	case "out":
		query = query.Where("from_user_id = ?", userID)
	case "in":
		query = query.Where("to_user_id = ?", userID)
	default: // "all" or empty
		query = query.Where("from_user_id = ? OR to_user_id = ?", userID, userID)
	}

	// Filter by asset
	if assetCode = strings.ToUpper(strings.TrimSpace(assetCode)); assetCode != "" {
		query = query.Where("asset_code = ?", assetCode)
	}

	// Filter by status
	if status = strings.ToLower(strings.TrimSpace(status)); status != "" {
		query = query.Where("status = ?", status)
	}

	// Count total
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Pagination
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize

	var items []*model.CurrencyTransferOrderModel
	err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&items).Error
	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
}
