package repository

import (
	"context"
	"fmt"
	"strings"

	"internalwallet/services/admin/rpc/internal/model"

	"gorm.io/gorm"
)

type CurrencyChainSettingsRepository interface {
	WithTx(tx *gorm.DB) CurrencyChainSettingsRepository
	GetDB() *gorm.DB

	ListByAssetCode(ctx context.Context, assetCode string) ([]*model.CurrencyChainSettingsModel, error)
	FindByAssetAndChain(ctx context.Context, assetCode, chainCode string) (*model.CurrencyChainSettingsModel, error)
	ReplaceForAsset(ctx context.Context, assetCode string, updatedBy int64, items []*model.CurrencyChainSettingsModel) error
}

type currencyChainSettingsRepo struct{ db *gorm.DB }

func NewCurrencyChainSettingsRepository(db *gorm.DB) CurrencyChainSettingsRepository {
	return &currencyChainSettingsRepo{db: db}
}
func (r *currencyChainSettingsRepo) WithTx(tx *gorm.DB) CurrencyChainSettingsRepository {
	return &currencyChainSettingsRepo{db: tx}
}
func (r *currencyChainSettingsRepo) GetDB() *gorm.DB { return r.db }

func (r *currencyChainSettingsRepo) ListByAssetCode(ctx context.Context, assetCode string) ([]*model.CurrencyChainSettingsModel, error) {
	assetCode = strings.ToUpper(strings.TrimSpace(assetCode))
	if assetCode == "" {
		return []*model.CurrencyChainSettingsModel{}, nil
	}
	var items []*model.CurrencyChainSettingsModel
	err := r.db.WithContext(ctx).
		Where("asset_code = ?", assetCode).
		Order("chain_code ASC").
		Find(&items).Error
	return items, err
}

func (r *currencyChainSettingsRepo) FindByAssetAndChain(ctx context.Context, assetCode, chainCode string) (*model.CurrencyChainSettingsModel, error) {
	assetCode = strings.ToUpper(strings.TrimSpace(assetCode))
	chainCode = strings.ToUpper(strings.TrimSpace(chainCode))

	if assetCode == "" || chainCode == "" {
		return nil, fmt.Errorf("asset_code and chain_code are required")
	}

	var item model.CurrencyChainSettingsModel
	err := r.db.WithContext(ctx).
		Where("asset_code = ? AND chain_code = ?", assetCode, chainCode).
		First(&item).Error

	if err != nil {
		return nil, err
	}

	return &item, nil
}

func (r *currencyChainSettingsRepo) ReplaceForAsset(ctx context.Context, assetCode string, updatedBy int64, items []*model.CurrencyChainSettingsModel) error {
	assetCode = strings.ToUpper(strings.TrimSpace(assetCode))
	if assetCode == "" {
		return fmt.Errorf("invalid asset")
	}
	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		}
	}()

	// Sync strategy:
	// - Existing rows: update by id
	// - New rows: create when id=0
	// - Removed rows (not present in request): hard delete (no deleted_at history)

	var existing []model.CurrencyChainSettingsModel
	if err := tx.
		Where("asset_code = ?", assetCode).
		Find(&existing).Error; err != nil {
		tx.Rollback()
		return err
	}

	existingByID := make(map[int64]model.CurrencyChainSettingsModel, len(existing))
	for _, e := range existing {
		existingByID[e.ID] = e
	}

	keepIDs := map[int64]struct{}{}
	for _, it := range items {
		if it == nil {
			continue
		}
		if it.ID > 0 {
			keepIDs[it.ID] = struct{}{}
		}
	}

	// Hard-delete rows removed from request.
	deleteIDs := make([]int64, 0)
	for _, e := range existing {
		if _, ok := keepIDs[e.ID]; ok {
			continue
		}
		deleteIDs = append(deleteIDs, e.ID)
	}
	if len(deleteIDs) > 0 {
		if err := tx.Unscoped().
			Where("asset_code = ? AND id IN ?", assetCode, deleteIDs).
			Delete(&model.CurrencyChainSettingsModel{}).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	for _, it := range items {
		if it == nil {
			continue
		}
		it.AssetCode = assetCode
		it.ChainCode = strings.ToUpper(strings.TrimSpace(it.ChainCode))
		it.UpdatedBy = updatedBy

		if it.ID > 0 {
			old, ok := existingByID[it.ID]
			if !ok {
				tx.Rollback()
				return fmt.Errorf("currency_chain_settings not found (asset_code=%s, id=%d)", assetCode, it.ID)
			}
			// Keep chain_code immutable for existing rows to avoid uniqueness conflicts and unclear semantics.
			if strings.ToUpper(strings.TrimSpace(old.ChainCode)) != it.ChainCode {
				tx.Rollback()
				return fmt.Errorf("chain_code cannot be changed for existing currency_chain_settings row (id=%d)", it.ID)
			}

			updates := map[string]any{
				"deposit_enabled":            it.DepositEnabled,
				"withdraw_enabled":           it.WithdrawEnabled,
				"web3_asset_display_enabled": it.Web3AssetDisplayEnabled,
				"consolidation_enabled":      it.ConsolidationEnabled,
				"status":                     it.Status,
				"contract_address":           it.ContractAddress,
				"token_decimals":             it.TokenDecimals,
				"min_withdraw_amount":        it.MinWithdrawAmount,
				"min_deposit_amount":         it.MinDepositAmount,
				"updated_by":                 updatedBy,
			}

			if err := tx.Model(&model.CurrencyChainSettingsModel{}).
				Where("asset_code = ? AND id = ?", assetCode, it.ID).
				Updates(updates).Error; err != nil {
				tx.Rollback()
				return err
			}
			continue
		}

		// New row (id=0)
		it.ID = 0
		if err := tx.Create(it).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit().Error
}
