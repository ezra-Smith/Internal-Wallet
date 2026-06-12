package repository

import (
	"context"
	"fmt"
	"strings"

	"internalwallet/services/admin/rpc/internal/model"

	"gorm.io/gorm"
)

type CurrencyWithdrawFeeRuleRepository interface {
	WithTx(tx *gorm.DB) CurrencyWithdrawFeeRuleRepository
	GetDB() *gorm.DB

	ListByAssetChain(ctx context.Context, assetCode, chainCode string) ([]*model.CurrencyWithdrawFeeRuleModel, error)
	ReplaceForAssetChain(ctx context.Context, assetCode, chainCode string, updatedBy int64, rules []*model.CurrencyWithdrawFeeRuleModel) error
}

type currencyWithdrawFeeRuleRepo struct{ db *gorm.DB }

func NewCurrencyWithdrawFeeRuleRepository(db *gorm.DB) CurrencyWithdrawFeeRuleRepository {
	return &currencyWithdrawFeeRuleRepo{db: db}
}
func (r *currencyWithdrawFeeRuleRepo) WithTx(tx *gorm.DB) CurrencyWithdrawFeeRuleRepository {
	return &currencyWithdrawFeeRuleRepo{db: tx}
}
func (r *currencyWithdrawFeeRuleRepo) GetDB() *gorm.DB { return r.db }

func (r *currencyWithdrawFeeRuleRepo) ListByAssetChain(ctx context.Context, assetCode, chainCode string) ([]*model.CurrencyWithdrawFeeRuleModel, error) {
	assetCode = strings.ToUpper(strings.TrimSpace(assetCode))
	chainCode = strings.ToUpper(strings.TrimSpace(chainCode))
	if assetCode == "" || chainCode == "" {
		return []*model.CurrencyWithdrawFeeRuleModel{}, nil
	}
	var items []*model.CurrencyWithdrawFeeRuleModel
	err := r.db.WithContext(ctx).
		Where("asset_code = ? AND chain_code = ?", assetCode, chainCode).
		Order("sort_order ASC, id ASC").
		Find(&items).Error
	return items, err
}

func (r *currencyWithdrawFeeRuleRepo) ReplaceForAssetChain(ctx context.Context, assetCode, chainCode string, updatedBy int64, rules []*model.CurrencyWithdrawFeeRuleModel) error {
	assetCode = strings.ToUpper(strings.TrimSpace(assetCode))
	chainCode = strings.ToUpper(strings.TrimSpace(chainCode))
	if assetCode == "" || chainCode == "" {
		return fmt.Errorf("invalid asset/chain")
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

	// Soft-delete existing active rules (idempotent; uniqueness is enforced for active rows only).
	if err := tx.
		Where("asset_code = ? AND chain_code = ?", assetCode, chainCode).
		Delete(&model.CurrencyWithdrawFeeRuleModel{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	for _, it := range rules {
		if it == nil {
			continue
		}
		it.AssetCode = assetCode
		it.ChainCode = chainCode
		it.RuleType = strings.ToLower(strings.TrimSpace(it.RuleType))
		it.UpdatedBy = updatedBy
		if err := tx.Create(it).Error; err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit().Error
}
