package repository

import (
	"context"
	"errors"
	"fmt"

	"internalwallet/services/admin/rpc/internal/model"

	"gorm.io/gorm"
)

type VaultAddressBalanceRepository interface {
	WithTx(tx *gorm.DB) VaultAddressBalanceRepository
	GetDB() *gorm.DB

	Upsert(ctx context.Context, balance *model.VaultAddressBalanceModel) error
	FindByAddressID(ctx context.Context, addressID int64) ([]*model.VaultAddressBalanceModel, error)
	FindByAddressAndCurrency(ctx context.Context, addressID int64, currency, contractAddress string) (*model.VaultAddressBalanceModel, error)
	GetTotalBalanceUSD(ctx context.Context, addressID int64) (int64, error)
	GetCurrenciesCount(ctx context.Context, addressID int64) (int32, error)
}

type vaultAddressBalanceRepo struct {
	db *gorm.DB
}

func NewVaultAddressBalanceRepository(db *gorm.DB) VaultAddressBalanceRepository {
	return &vaultAddressBalanceRepo{db: db}
}

func (r *vaultAddressBalanceRepo) WithTx(tx *gorm.DB) VaultAddressBalanceRepository {
	return &vaultAddressBalanceRepo{db: tx}
}

func (r *vaultAddressBalanceRepo) GetDB() *gorm.DB {
	return r.db
}

func (r *vaultAddressBalanceRepo) Upsert(ctx context.Context, balance *model.VaultAddressBalanceModel) error {
	// 使用 GORM 的 Clauses(clause.OnConflict{...}) 或者先查询再更新
	var existing model.VaultAddressBalanceModel
	err := r.db.WithContext(ctx).
		Where("address_id = ? AND network_id = ? AND currency = ? AND contract_address = ? AND deleted_at IS NULL",
			balance.AddressID, balance.NetworkID, balance.Currency, balance.ContractAddress).
		First(&existing).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 不存在,创建新记录
		return r.db.WithContext(ctx).Create(balance).Error
	}
	if err != nil {
		return err
	}

	// 存在,更新记录
	return r.db.WithContext(ctx).
		Model(&model.VaultAddressBalanceModel{}).
		Where("id = ?", existing.ID).
		Updates(map[string]interface{}{
			"balance":         balance.Balance,
			"balance_raw":     balance.BalanceRaw,
			"balance_usd":     balance.BalanceUSD,
			"balance_usd_raw": balance.BalanceUSDRaw,
			"last_synced_at":  balance.LastSyncedAt,
		}).Error
}

func (r *vaultAddressBalanceRepo) FindByAddressID(ctx context.Context, addressID int64) ([]*model.VaultAddressBalanceModel, error) {
	if addressID == 0 {
		return nil, fmt.Errorf("invalid address id")
	}
	var items []*model.VaultAddressBalanceModel
	err := r.db.WithContext(ctx).
		Where("address_id = ? AND deleted_at IS NULL", addressID).
		Order("balance_usd_raw DESC").
		Find(&items).Error
	return items, err
}

func (r *vaultAddressBalanceRepo) FindByAddressAndCurrency(ctx context.Context, addressID int64, currency, contractAddress string) (*model.VaultAddressBalanceModel, error) {
	var m model.VaultAddressBalanceModel
	err := r.db.WithContext(ctx).
		Where("address_id = ? AND currency = ? AND contract_address = ? AND deleted_at IS NULL",
			addressID, currency, contractAddress).
		First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &m, err
}

func (r *vaultAddressBalanceRepo) GetTotalBalanceUSD(ctx context.Context, addressID int64) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).
		Model(&model.VaultAddressBalanceModel{}).
		Where("address_id = ? AND deleted_at IS NULL", addressID).
		Select("COALESCE(SUM(balance_usd_raw), 0)").
		Scan(&total).Error
	return total, err
}

func (r *vaultAddressBalanceRepo) GetCurrenciesCount(ctx context.Context, addressID int64) (int32, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.VaultAddressBalanceModel{}).
		Where("address_id = ? AND deleted_at IS NULL AND balance_raw > 0", addressID).
		Count(&count).Error
	return int32(count), err
}
