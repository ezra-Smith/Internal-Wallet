package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"internalwallet/services/admin/rpc/internal/model"

	"gorm.io/gorm"
)

type CurrencyWithdrawOrderListFilter struct {
	UserID      int64
	AssetCode   string
	ChainCode   string
	Statuses    []string
	Strategies  []string
	ToAddress   string
	TxHash      string
	CreatedFrom *time.Time
	CreatedTo   *time.Time
	MinAmount   *string
	MaxAmount   *string
	SortBy      string
	SortOrder   string
}

type CurrencyWithdrawOrderRepository interface {
	WithTx(tx *gorm.DB) CurrencyWithdrawOrderRepository
	GetDB() *gorm.DB

	Create(ctx context.Context, m *model.CurrencyWithdrawOrderModel) error
	FindByID(ctx context.Context, id int64) (*model.CurrencyWithdrawOrderModel, error)
	FindByTxHash(ctx context.Context, txHash string) (*model.CurrencyWithdrawOrderModel, error)
	List(ctx context.Context, page, pageSize int32, f CurrencyWithdrawOrderListFilter) ([]*model.CurrencyWithdrawOrderModel, int64, error)
	UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error
}

type currencyWithdrawOrderRepo struct{ db *gorm.DB }

func NewCurrencyWithdrawOrderRepository(db *gorm.DB) CurrencyWithdrawOrderRepository {
	return &currencyWithdrawOrderRepo{db: db}
}
func (r *currencyWithdrawOrderRepo) WithTx(tx *gorm.DB) CurrencyWithdrawOrderRepository {
	return &currencyWithdrawOrderRepo{db: tx}
}
func (r *currencyWithdrawOrderRepo) GetDB() *gorm.DB { return r.db }

func (r *currencyWithdrawOrderRepo) Create(ctx context.Context, m *model.CurrencyWithdrawOrderModel) error {
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *currencyWithdrawOrderRepo) FindByID(ctx context.Context, id int64) (*model.CurrencyWithdrawOrderModel, error) {
	var m model.CurrencyWithdrawOrderModel
	err := r.db.WithContext(ctx).First(&m, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("withdrawal not found")
	}
	return &m, err
}

func (r *currencyWithdrawOrderRepo) FindByTxHash(ctx context.Context, txHash string) (*model.CurrencyWithdrawOrderModel, error) {
	txHash = strings.TrimSpace(txHash)
	if txHash == "" {
		return nil, fmt.Errorf("withdrawal not found")
	}
	var m model.CurrencyWithdrawOrderModel
	err := r.db.WithContext(ctx).
		Where("tx_hash = ?", txHash).
		First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("withdrawal not found")
	}
	return &m, err
}

func (r *currencyWithdrawOrderRepo) List(ctx context.Context, page, pageSize int32, f CurrencyWithdrawOrderListFilter) ([]*model.CurrencyWithdrawOrderModel, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	query := r.db.WithContext(ctx).Model(&model.CurrencyWithdrawOrderModel{})
	if f.UserID > 0 {
		query = query.Where("user_id = ?", f.UserID)
	}
	if strings.TrimSpace(f.AssetCode) != "" {
		query = query.Where("asset_code = ?", strings.ToUpper(strings.TrimSpace(f.AssetCode)))
	}
	if strings.TrimSpace(f.ChainCode) != "" {
		query = query.Where("chain_code = ?", strings.ToUpper(strings.TrimSpace(f.ChainCode)))
	}
	if len(f.Statuses) > 0 {
		query = query.Where("status IN ?", f.Statuses)
	}
	if len(f.Strategies) > 0 {
		query = query.Where("strategy IN ?", f.Strategies)
	}
	if strings.TrimSpace(f.ToAddress) != "" {
		query = query.Where("to_address = ?", strings.TrimSpace(f.ToAddress))
	}
	if strings.TrimSpace(f.TxHash) != "" {
		query = query.Where("tx_hash = ?", strings.TrimSpace(f.TxHash))
	}
	if f.CreatedFrom != nil && !f.CreatedFrom.IsZero() {
		query = query.Where("created_at >= ?", *f.CreatedFrom)
	}
	if f.CreatedTo != nil && !f.CreatedTo.IsZero() {
		query = query.Where("created_at <= ?", *f.CreatedTo)
	}
	if f.MinAmount != nil {
		query = query.Where("amount >= ?", *f.MinAmount)
	}
	if f.MaxAmount != nil {
		query = query.Where("amount <= ?", *f.MaxAmount)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	orderCol := "created_at"
	switch strings.TrimSpace(f.SortBy) {
	case "", "created_at":
		orderCol = "created_at"
	case "amount":
		orderCol = "amount"
	case "status":
		orderCol = "status"
	}
	orderDir := "DESC"
	if strings.EqualFold(strings.TrimSpace(f.SortOrder), "asc") {
		orderDir = "ASC"
	}

	var items []*model.CurrencyWithdrawOrderModel
	offset := int((page - 1) * pageSize)
	err := query.Order(orderCol + " " + orderDir).Offset(offset).Limit(int(pageSize)).Find(&items).Error
	return items, total, err
}

func (r *currencyWithdrawOrderRepo) UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error {
	res := r.db.WithContext(ctx).
		Model(&model.CurrencyWithdrawOrderModel{}).
		Where("id = ?", id).
		Updates(fields)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("withdrawal not found")
	}
	return nil
}
