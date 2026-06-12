package repository

import (
	"context"
	"fmt"
	"strings"

	"internalwallet/services/admin/rpc/internal/model"

	"gorm.io/gorm"
)

type CurrencyWithdrawAuditRuleRepository interface {
	WithTx(tx *gorm.DB) CurrencyWithdrawAuditRuleRepository
	GetDB() *gorm.DB

	ListByAssetChain(ctx context.Context, assetCode, chainCode string) ([]*model.CurrencyWithdrawAuditRuleModel, error)
	ReplaceForAssetChain(ctx context.Context, assetCode, chainCode string, updatedBy int64, rules []*model.CurrencyWithdrawAuditRuleModel) error
}

type currencyWithdrawAuditRuleRepo struct{ db *gorm.DB }

func NewCurrencyWithdrawAuditRuleRepository(db *gorm.DB) CurrencyWithdrawAuditRuleRepository {
	return &currencyWithdrawAuditRuleRepo{db: db}
}
func (r *currencyWithdrawAuditRuleRepo) WithTx(tx *gorm.DB) CurrencyWithdrawAuditRuleRepository {
	return &currencyWithdrawAuditRuleRepo{db: tx}
}
func (r *currencyWithdrawAuditRuleRepo) GetDB() *gorm.DB { return r.db }

func (r *currencyWithdrawAuditRuleRepo) ListByAssetChain(ctx context.Context, assetCode, chainCode string) ([]*model.CurrencyWithdrawAuditRuleModel, error) {
	assetCode = strings.ToUpper(strings.TrimSpace(assetCode))
	chainCode = strings.ToUpper(strings.TrimSpace(chainCode))
	if assetCode == "" || chainCode == "" {
		return []*model.CurrencyWithdrawAuditRuleModel{}, nil
	}
	var items []*model.CurrencyWithdrawAuditRuleModel
	err := r.db.WithContext(ctx).
		Where("asset_code = ? AND chain_code = ?", assetCode, chainCode).
		Order("sort_order ASC, id ASC").
		Find(&items).Error
	return items, err
}

func (r *currencyWithdrawAuditRuleRepo) ReplaceForAssetChain(ctx context.Context, assetCode, chainCode string, updatedBy int64, rules []*model.CurrencyWithdrawAuditRuleModel) error {
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
		Delete(&model.CurrencyWithdrawAuditRuleModel{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	for _, it := range rules {
		if it == nil {
			continue
		}
		it.AssetCode = assetCode
		it.ChainCode = chainCode
		it.Strategy = strings.ToLower(strings.TrimSpace(it.Strategy))
		it.UpdatedBy = updatedBy
		if err := tx.Create(it).Error; err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit().Error
}
