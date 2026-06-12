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

type WalletDepositListFilter struct {
	ID              int64
	UserID          int64
	AssetCode       string
	ChainCode       string
	Statuses        []string
	DepositAddress  string
	TransactionHash string
	CreatedFrom     *time.Time
	CreatedTo       *time.Time
	MinAmount       *string
	MaxAmount       *string
	SortBy          string
	SortOrder       string
}

type WalletDepositRepository interface {
	WithTx(tx *gorm.DB) WalletDepositRepository
	GetDB() *gorm.DB

	Create(ctx context.Context, m *model.WalletDepositModel) error
	FindByID(ctx context.Context, id int64) (*model.WalletDepositModel, error)
	FindByTxHash(ctx context.Context, txHash string) (*model.WalletDepositModel, error)
	List(ctx context.Context, page, pageSize int32, f WalletDepositListFilter) ([]*model.WalletDepositModel, int64, error)
	UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error

	CountByDepositAddressAndStatuses(ctx context.Context, depositAddress string, statuses []string) (int64, error)
}

type walletDepositRepo struct {
	db *gorm.DB
}

func NewWalletDepositRepository(db *gorm.DB) WalletDepositRepository {
	return &walletDepositRepo{db: db}
}
func (r *walletDepositRepo) WithTx(tx *gorm.DB) WalletDepositRepository {
	return &walletDepositRepo{db: tx}
}
func (r *walletDepositRepo) GetDB() *gorm.DB { return r.db }

func (r *walletDepositRepo) Create(ctx context.Context, m *model.WalletDepositModel) error {
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *walletDepositRepo) FindByID(ctx context.Context, id int64) (*model.WalletDepositModel, error) {
	var m model.WalletDepositModel
	err := r.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", id).
		First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("deposit not found")
	}
	return &m, err
}

func (r *walletDepositRepo) FindByTxHash(ctx context.Context, txHash string) (*model.WalletDepositModel, error) {
	txHash = strings.TrimSpace(txHash)
	if txHash == "" {
		return nil, fmt.Errorf("deposit not found")
	}
	var m model.WalletDepositModel
	err := r.db.WithContext(ctx).
		Where("transaction_hash = ? AND deleted_at IS NULL", txHash).
		First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("deposit not found")
	}
	return &m, err
}

func (r *walletDepositRepo) List(ctx context.Context, page, pageSize int32, f WalletDepositListFilter) ([]*model.WalletDepositModel, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	query := r.db.WithContext(ctx).Model(&model.WalletDepositModel{}).Where("deleted_at IS NULL")
	if f.ID > 0 {
		query = query.Where("id = ?", f.ID)
	}
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
	if strings.TrimSpace(f.DepositAddress) != "" {
		query = query.Where("deposit_address = ?", strings.TrimSpace(f.DepositAddress))
	}
	if strings.TrimSpace(f.TransactionHash) != "" {
		query = query.Where("transaction_hash = ?", strings.TrimSpace(f.TransactionHash))
	}
	if f.CreatedFrom != nil && !f.CreatedFrom.IsZero() {
		query = query.Where("created_at >= ?", *f.CreatedFrom)
	}
	if f.CreatedTo != nil && !f.CreatedTo.IsZero() {
		query = query.Where("created_at <= ?", *f.CreatedTo)
	}
	if f.MinAmount != nil {
		query = query.Where("amount >= ?", strings.TrimSpace(*f.MinAmount))
	}
	if f.MaxAmount != nil {
		query = query.Where("amount <= ?", strings.TrimSpace(*f.MaxAmount))
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

	var items []*model.WalletDepositModel
	offset := int((page - 1) * pageSize)
	err := query.Order(orderCol + " " + orderDir).Offset(offset).Limit(int(pageSize)).Find(&items).Error
	return items, total, err
}

func (r *walletDepositRepo) UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error {
	res := r.db.WithContext(ctx).
		Model(&model.WalletDepositModel{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(fields)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("deposit not found")
	}
	return nil
}

func (r *walletDepositRepo) CountByDepositAddressAndStatuses(ctx context.Context, depositAddress string, statuses []string) (int64, error) {
	depositAddress = strings.TrimSpace(depositAddress)
	if depositAddress == "" {
		return 0, nil
	}
	q := r.db.WithContext(ctx).Model(&model.WalletDepositModel{}).
		Where("deposit_address = ? AND deleted_at IS NULL", depositAddress)
	if len(statuses) > 0 {
		q = q.Where("status IN ?", statuses)
	}
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}
