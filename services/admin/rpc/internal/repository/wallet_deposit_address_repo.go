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

// DepositAddressWithBalance 充值地址+余额联合查询结果
type DepositAddressWithBalance struct {
	model.WalletDepositAddressModel
	// 注意：wallet_user_chain_addresses 不再存 asset_code；如需展示/筛选资产，使用余额表的 asset_code。
	AssetCode *string `gorm:"column:asset_code"`
	// 余额信息（来自LEFT JOIN wallet_deposit_address_balances）
	Balance       *string    `gorm:"column:balance"`
	BalanceRaw    *string    `gorm:"column:balance_raw"` // DECIMAL(65,0) in DB; keep as string to avoid int64 overflow
	BalanceUSD    *string    `gorm:"column:balance_usd"`
	BalanceUSDRaw *string    `gorm:"column:balance_usd_raw"` // DECIMAL(65,0) in DB; keep as string to avoid int64 overflow
	NeedsSweep    *int8      `gorm:"column:needs_sweep"`
	LastSyncedAt  *time.Time `gorm:"column:last_synced_at"`
	SyncStatus    *string    `gorm:"column:sync_status"`
}

type WalletDepositAddressRepository interface {
	WithTx(tx *gorm.DB) WalletDepositAddressRepository
	GetDB() *gorm.DB

	Create(ctx context.Context, m *model.WalletDepositAddressModel) error
	FindByID(ctx context.Context, id int64) (*model.WalletDepositAddressModel, error)
	FindByAddress(ctx context.Context, address string) (*model.WalletDepositAddressModel, error)
	FindActiveByUserChain(ctx context.Context, userID int64, chainCode string) (*model.WalletDepositAddressModel, error)
	List(ctx context.Context, page, pageSize int32, userID int64, chainCode, status, address, sortBy, sortOrder string) ([]*model.WalletDepositAddressModel, int64, error)
	ListWithBalance(ctx context.Context, page, pageSize int32, userID int64, assetCode, chainCode, status, address, sortBy, sortOrder string) ([]*DepositAddressWithBalance, int64, error)
	UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error
	SoftDelete(ctx context.Context, id int64, now time.Time) error
}

type walletDepositAddressRepo struct {
	db *gorm.DB
}

func NewWalletDepositAddressRepository(db *gorm.DB) WalletDepositAddressRepository {
	return &walletDepositAddressRepo{db: db}
}

func (r *walletDepositAddressRepo) WithTx(tx *gorm.DB) WalletDepositAddressRepository {
	return &walletDepositAddressRepo{db: tx}
}

func (r *walletDepositAddressRepo) GetDB() *gorm.DB { return r.db }

func (r *walletDepositAddressRepo) Create(ctx context.Context, m *model.WalletDepositAddressModel) error {
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *walletDepositAddressRepo) FindByID(ctx context.Context, id int64) (*model.WalletDepositAddressModel, error) {
	var m model.WalletDepositAddressModel
	err := r.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", id).
		First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("deposit address not found")
	}
	return &m, err
}

func (r *walletDepositAddressRepo) FindByAddress(ctx context.Context, address string) (*model.WalletDepositAddressModel, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		return nil, fmt.Errorf("deposit address not found")
	}
	var m model.WalletDepositAddressModel
	err := r.db.WithContext(ctx).
		Where("address = ? AND deleted_at IS NULL", address).
		First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("deposit address not found")
	}
	return &m, err
}

func (r *walletDepositAddressRepo) FindActiveByUserChain(ctx context.Context, userID int64, chainCode string) (*model.WalletDepositAddressModel, error) {
	chainCode = strings.ToUpper(strings.TrimSpace(chainCode))
	if userID <= 0 || chainCode == "" {
		return nil, fmt.Errorf("deposit address not found")
	}
	var m model.WalletDepositAddressModel
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND chain_code = ? AND status = ? AND deleted_at IS NULL", userID, chainCode, "active").
		First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("deposit address not found")
	}
	return &m, err
}

func (r *walletDepositAddressRepo) List(ctx context.Context, page, pageSize int32, userID int64, chainCode, status, address, sortBy, sortOrder string) ([]*model.WalletDepositAddressModel, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	query := r.db.WithContext(ctx).Model(&model.WalletDepositAddressModel{}).Where("deleted_at IS NULL")
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}
	if strings.TrimSpace(chainCode) != "" {
		query = query.Where("chain_code = ?", strings.ToUpper(strings.TrimSpace(chainCode)))
	}
	if strings.TrimSpace(status) != "" {
		query = query.Where("status = ?", strings.TrimSpace(status))
	}
	if strings.TrimSpace(address) != "" {
		query = query.Where("address = ?", strings.TrimSpace(address))
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	orderCol := "id"
	switch strings.TrimSpace(sortBy) {
	case "", "created_at":
		orderCol = "created_at"
	case "id":
		orderCol = "id"
	}
	orderDir := "DESC"
	if strings.EqualFold(strings.TrimSpace(sortOrder), "asc") {
		orderDir = "ASC"
	}

	var items []*model.WalletDepositAddressModel
	offset := int((page - 1) * pageSize)
	err := query.Order(orderCol + " " + orderDir).Offset(offset).Limit(int(pageSize)).Find(&items).Error
	return items, total, err
}

func (r *walletDepositAddressRepo) UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error {
	res := r.db.WithContext(ctx).
		Model(&model.WalletDepositAddressModel{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(fields)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("deposit address not found")
	}
	return nil
}

func (r *walletDepositAddressRepo) ListWithBalance(ctx context.Context, page, pageSize int32, userID int64, assetCode, chainCode, status, address, sortBy, sortOrder string) ([]*DepositAddressWithBalance, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	// 构建LEFT JOIN查询
	query := r.db.WithContext(ctx).
		Table("wallet_user_chain_addresses AS addr").
		Select(`
			addr.id,
			addr.user_id,
			addr.chain_code,
			addr.address,
			addr.status,
			addr.memo,
			addr.created_at,
			addr.updated_at,
			addr.deleted_at,
			bal.asset_code AS asset_code,
			bal.balance,
			bal.balance_raw,
			bal.balance_usd,
			bal.balance_usd_raw,
			bal.needs_sweep,
			bal.last_synced_at,
			bal.sync_status
		`).
		Joins("LEFT JOIN wallet_deposit_address_balances AS bal ON addr.id = bal.deposit_address_id AND bal.deleted_at IS NULL").
		Where("addr.deleted_at IS NULL")

	// 添加筛选条件
	if userID > 0 {
		query = query.Where("addr.user_id = ?", userID)
	}
	if strings.TrimSpace(assetCode) != "" {
		query = query.Where("bal.asset_code = ?", strings.ToUpper(strings.TrimSpace(assetCode)))
	}
	if strings.TrimSpace(chainCode) != "" {
		query = query.Where("addr.chain_code = ?", strings.ToUpper(strings.TrimSpace(chainCode)))
	}
	if strings.TrimSpace(status) != "" {
		query = query.Where("addr.status = ?", strings.TrimSpace(status))
	}
	if strings.TrimSpace(address) != "" {
		query = query.Where("addr.address = ?", strings.TrimSpace(address))
	}

	// 计算总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 排序
	orderCol := "addr.id"
	switch strings.TrimSpace(sortBy) {
	case "", "created_at":
		orderCol = "addr.created_at"
	case "id":
		orderCol = "addr.id"
	}
	orderDir := "DESC"
	if strings.EqualFold(strings.TrimSpace(sortOrder), "asc") {
		orderDir = "ASC"
	}

	// 查询数据
	var items []*DepositAddressWithBalance
	offset := int((page - 1) * pageSize)
	err := query.Order(orderCol + " " + orderDir).Offset(offset).Limit(int(pageSize)).Scan(&items).Error
	return items, total, err
}

func (r *walletDepositAddressRepo) SoftDelete(ctx context.Context, id int64, now time.Time) error {
	res := r.db.WithContext(ctx).
		Model(&model.WalletDepositAddressModel{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]interface{}{
			"deleted_at": &now,
			"updated_at": &now,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("deposit address not found")
	}
	return nil
}
