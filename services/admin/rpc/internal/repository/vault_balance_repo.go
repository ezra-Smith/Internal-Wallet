package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"internalwallet/services/admin/rpc/internal/model"

	"gorm.io/gorm"
)

type VaultBalanceRepository interface {
	WithTx(tx *gorm.DB) VaultBalanceRepository
	GetDB() *gorm.DB

	FindByNetworkCurrencyContract(ctx context.Context, networkID int64, currency, contractAddress string) (*model.VaultBalanceModel, error)
	ListByNetworkID(ctx context.Context, networkID int64) ([]*model.VaultBalanceModel, error)
	Upsert(ctx context.Context, m *model.VaultBalanceModel) error
}

type vaultBalanceRepo struct {
	db *gorm.DB
}

func NewVaultBalanceRepository(db *gorm.DB) VaultBalanceRepository { return &vaultBalanceRepo{db: db} }
func (r *vaultBalanceRepo) WithTx(tx *gorm.DB) VaultBalanceRepository {
	return &vaultBalanceRepo{db: tx}
}
func (r *vaultBalanceRepo) GetDB() *gorm.DB { return r.db }

func (r *vaultBalanceRepo) FindByNetworkCurrencyContract(ctx context.Context, networkID int64, currency, contractAddress string) (*model.VaultBalanceModel, error) {
	currency = strings.TrimSpace(currency)
	contractAddress = strings.TrimSpace(contractAddress)
	if networkID <= 0 || currency == "" {
		return nil, fmt.Errorf("balance not found")
	}
	var m model.VaultBalanceModel
	err := r.db.WithContext(ctx).
		Where("network_id = ? AND currency = ? AND contract_address = ? AND deleted_at IS NULL", networkID, currency, contractAddress).
		First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("balance not found")
	}
	return &m, err
}

func (r *vaultBalanceRepo) ListByNetworkID(ctx context.Context, networkID int64) ([]*model.VaultBalanceModel, error) {
	if networkID <= 0 {
		return nil, nil
	}
	var items []*model.VaultBalanceModel
	err := r.db.WithContext(ctx).
		Where("network_id = ? AND deleted_at IS NULL", networkID).
		Order("currency ASC").
		Find(&items).Error
	return items, err
}

func (r *vaultBalanceRepo) Upsert(ctx context.Context, m *model.VaultBalanceModel) error {
	if m == nil || m.NetworkID <= 0 || strings.TrimSpace(m.Currency) == "" {
		return fmt.Errorf("invalid balance")
	}
	m.Currency = strings.TrimSpace(m.Currency)
	m.ContractAddress = strings.TrimSpace(m.ContractAddress)

	updates := map[string]interface{}{
		"currency":         m.Currency,
		"contract_address": m.ContractAddress,
		"balance":          m.Balance,
		"balance_raw":      m.BalanceRaw,
		"balance_usd":      m.BalanceUSD,
		"balance_usd_raw":  m.BalanceUSDRaw,
		"last_updated_at":  m.LastUpdatedAt,
		"updated_at":       m.UpdatedAt,
	}
	res := r.db.WithContext(ctx).
		Model(&model.VaultBalanceModel{}).
		Where("network_id = ? AND currency = ? AND contract_address = ? AND deleted_at IS NULL", m.NetworkID, m.Currency, m.ContractAddress).
		Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected > 0 {
		return nil
	}

	// Best-effort create; on race, fall back to update.
	if err := r.db.WithContext(ctx).Create(m).Error; err != nil {
		res := r.db.WithContext(ctx).
			Model(&model.VaultBalanceModel{}).
			Where("network_id = ? AND currency = ? AND contract_address = ? AND deleted_at IS NULL", m.NetworkID, m.Currency, m.ContractAddress).
			Updates(updates)
		return res.Error
	}
	return nil
}
