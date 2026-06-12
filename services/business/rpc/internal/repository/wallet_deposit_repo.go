package repository

import (
	"context"
	"fmt"
	"strings"

	"internalwallet/services/business/rpc/internal/model"

	"gorm.io/gorm"
)

type DepositReceiveStat struct {
	ChainCode      string `gorm:"column:chain_code"`
	DepositAddress string `gorm:"column:deposit_address"`
	TotalAmount    string `gorm:"column:total_amount"`
	Count          int64  `gorm:"column:cnt"`
}

type WalletDepositRepository interface {
	WithTx(tx *gorm.DB) WalletDepositRepository
	GetDB() *gorm.DB

	CountByUserChainDepositAddressAndStatuses(ctx context.Context, userID int64, chainCode string, depositAddress string, statuses []string) (int64, error)
	ListByUserChainDepositAddress(ctx context.Context, userID int64, chainCode string, depositAddress string, assetCode string, page, pageSize int32) ([]*model.WalletDepositModel, int64, error)
	ListReceiveStatsByUser(ctx context.Context, userID int64, chainCode string, assetCode string, statuses []string) ([]DepositReceiveStat, error)
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

func (r *walletDepositRepo) CountByUserChainDepositAddressAndStatuses(ctx context.Context, userID int64, chainCode string, depositAddress string, statuses []string) (int64, error) {
	chainCode = strings.ToUpper(strings.TrimSpace(chainCode))
	depositAddress = strings.TrimSpace(depositAddress)
	if userID <= 0 || chainCode == "" || depositAddress == "" {
		return 0, nil
	}

	q := r.db.WithContext(ctx).Model(&model.WalletDepositModel{}).
		Where("user_id = ? AND chain_code = ? AND deleted_at IS NULL", userID, chainCode)

	if strings.HasPrefix(strings.ToLower(depositAddress), "0x") {
		q = q.Where("LOWER(deposit_address) = LOWER(?)", depositAddress)
	} else {
		q = q.Where("deposit_address = ?", depositAddress)
	}

	if len(statuses) > 0 {
		q = q.Where("status IN ?", statuses)
	}

	var count int64
	if err := q.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *walletDepositRepo) ListByUserChainDepositAddress(ctx context.Context, userID int64, chainCode string, depositAddress string, assetCode string, page, pageSize int32) ([]*model.WalletDepositModel, int64, error) {
	chainCode = strings.ToUpper(strings.TrimSpace(chainCode))
	depositAddress = strings.TrimSpace(depositAddress)
	assetCode = strings.ToUpper(strings.TrimSpace(assetCode))
	if userID <= 0 || chainCode == "" || depositAddress == "" {
		return nil, 0, nil
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	q := r.db.WithContext(ctx).Model(&model.WalletDepositModel{}).
		Where("user_id = ? AND chain_code = ? AND deleted_at IS NULL", userID, chainCode)
	if assetCode != "" {
		q = q.Where("asset_code = ?", assetCode)
	}
	if strings.HasPrefix(strings.ToLower(depositAddress), "0x") {
		q = q.Where("LOWER(deposit_address) = LOWER(?)", depositAddress)
	} else {
		q = q.Where("deposit_address = ?", depositAddress)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []*model.WalletDepositModel
	offset := int((page - 1) * pageSize)
	err := q.Order("created_at DESC").Offset(offset).Limit(int(pageSize)).Find(&items).Error
	return items, total, err
}

func (r *walletDepositRepo) ListReceiveStatsByUser(ctx context.Context, userID int64, chainCode string, assetCode string, statuses []string) ([]DepositReceiveStat, error) {
	chainCode = strings.ToUpper(strings.TrimSpace(chainCode))
	assetCode = strings.ToUpper(strings.TrimSpace(assetCode))
	if userID <= 0 {
		return nil, nil
	}

	q := r.db.WithContext(ctx).Table("wallet_deposits").
		Select("chain_code, deposit_address, CAST(COALESCE(SUM(amount), 0) AS CHAR) AS total_amount, COUNT(*) AS cnt").
		Where("user_id = ? AND deleted_at IS NULL", userID)
	if chainCode != "" {
		q = q.Where("chain_code = ?", chainCode)
	}
	if assetCode != "" {
		q = q.Where("asset_code = ?", assetCode)
	}
	if len(statuses) > 0 {
		q = q.Where("status IN ?", statuses)
	}

	var rows []DepositReceiveStat
	if err := q.Group("chain_code, deposit_address").Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("query deposit stats failed: %w", err)
	}
	return rows, nil
}
