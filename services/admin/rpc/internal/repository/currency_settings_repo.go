package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"internalwallet/services/admin/rpc/internal/model"

	"gorm.io/gorm"
)

type CurrencySettingsRepository interface {
	WithTx(tx *gorm.DB) CurrencySettingsRepository
	GetDB() *gorm.DB

	GetByAssetCode(ctx context.Context, assetCode string) (*model.CurrencySettingsModel, error)
	Upsert(ctx context.Context, m *model.CurrencySettingsModel) error
}

type currencySettingsRepo struct{ db *gorm.DB }

func NewCurrencySettingsRepository(db *gorm.DB) CurrencySettingsRepository {
	return &currencySettingsRepo{db: db}
}
func (r *currencySettingsRepo) WithTx(tx *gorm.DB) CurrencySettingsRepository {
	return &currencySettingsRepo{db: tx}
}
func (r *currencySettingsRepo) GetDB() *gorm.DB { return r.db }

func (r *currencySettingsRepo) GetByAssetCode(ctx context.Context, assetCode string) (*model.CurrencySettingsModel, error) {
	assetCode = strings.ToUpper(strings.TrimSpace(assetCode))
	if assetCode == "" {
		return nil, fmt.Errorf("currency settings not found")
	}
	var m model.CurrencySettingsModel
	err := r.db.WithContext(ctx).
		Where("asset_code = ?", assetCode).
		First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("currency settings not found")
	}
	return &m, err
}

func (r *currencySettingsRepo) Upsert(ctx context.Context, m *model.CurrencySettingsModel) error {
	if m == nil || strings.TrimSpace(m.AssetCode) == "" {
		return fmt.Errorf("invalid params")
	}
	m.AssetCode = strings.ToUpper(strings.TrimSpace(m.AssetCode))

	// Try update existing row first.
	res := r.db.WithContext(ctx).
		Model(&model.CurrencySettingsModel{}).
		Where("asset_code = ?", m.AssetCode).
		Updates(map[string]interface{}{
			"web2_deposit_enabled":      m.Web2DepositEnabled,
			"web2_withdraw_enabled":     m.Web2WithdrawEnabled,
			"web2_transfer_enabled":     m.Web2TransferEnabled,
			"web3_deposit_enabled":      m.Web3DepositEnabled,
			"web3_withdraw_enabled":     m.Web3WithdrawEnabled,
			"web3_swap_enabled":         m.Web3SwapEnabled,
			"use_global_withdraw_fee":   m.UseGlobalWithdrawFee,
			"use_global_withdraw_audit": m.UseGlobalWithdrawAudit,
			"use_global_transfer_audit": m.UseGlobalTransferAudit, // 添加缺失的字段
			"updated_by":                m.UpdatedBy,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected > 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(m).Error
}
