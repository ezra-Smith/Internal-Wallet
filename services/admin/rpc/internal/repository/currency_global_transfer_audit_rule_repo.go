package repository

import (
	"context"
	"strings"

	"internalwallet/services/admin/rpc/internal/model"

	"gorm.io/gorm"
)

type CurrencyGlobalTransferAuditRuleRepository interface {
	WithTx(tx *gorm.DB) CurrencyGlobalTransferAuditRuleRepository
	GetDB() *gorm.DB

	// ListAll returns all rules ordered by asset_code ASC, sort_order ASC.
	ListAll(ctx context.Context) ([]*model.CurrencyGlobalTransferAuditRuleModel, error)
	// ListByAsset returns rules for a specific asset.
	ListByAsset(ctx context.Context, assetCode string) ([]*model.CurrencyGlobalTransferAuditRuleModel, error)
	// ReplaceByAsset replaces all rules for a given asset.
	ReplaceByAsset(ctx context.Context, assetCode string, updatedBy int64, rules []*model.CurrencyGlobalTransferAuditRuleModel) error
	// ReplaceAll replaces all rules (grouped by asset_code in input).
	ReplaceAll(ctx context.Context, updatedBy int64, rulesByAsset map[string][]*model.CurrencyGlobalTransferAuditRuleModel) error
}

type currencyGlobalTransferAuditRuleRepo struct{ db *gorm.DB }

func NewCurrencyGlobalTransferAuditRuleRepository(db *gorm.DB) CurrencyGlobalTransferAuditRuleRepository {
	return &currencyGlobalTransferAuditRuleRepo{db: db}
}
func (r *currencyGlobalTransferAuditRuleRepo) WithTx(tx *gorm.DB) CurrencyGlobalTransferAuditRuleRepository {
	return &currencyGlobalTransferAuditRuleRepo{db: tx}
}
func (r *currencyGlobalTransferAuditRuleRepo) GetDB() *gorm.DB { return r.db }

func (r *currencyGlobalTransferAuditRuleRepo) ListAll(ctx context.Context) ([]*model.CurrencyGlobalTransferAuditRuleModel, error) {
	var items []*model.CurrencyGlobalTransferAuditRuleModel
	err := r.db.WithContext(ctx).
		Order("asset_code ASC, sort_order ASC, id ASC").
		Find(&items).Error
	return items, err
}

func (r *currencyGlobalTransferAuditRuleRepo) ListByAsset(ctx context.Context, assetCode string) ([]*model.CurrencyGlobalTransferAuditRuleModel, error) {
	ac := strings.ToUpper(strings.TrimSpace(assetCode))
	var items []*model.CurrencyGlobalTransferAuditRuleModel
	err := r.db.WithContext(ctx).
		Where("asset_code = ?", ac).
		Order("sort_order ASC, id ASC").
		Find(&items).Error
	return items, err
}

func (r *currencyGlobalTransferAuditRuleRepo) ReplaceByAsset(ctx context.Context, assetCode string, updatedBy int64, rules []*model.CurrencyGlobalTransferAuditRuleModel) error {
	ac := strings.ToUpper(strings.TrimSpace(assetCode))
	if ac == "" {
		return nil
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

	// Soft-delete existing active rules for this asset
	if err := tx.
		Where("asset_code = ? AND deleted_at IS NULL", ac).
		Delete(&model.CurrencyGlobalTransferAuditRuleModel{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	for _, it := range rules {
		if it == nil {
			continue
		}
		it.AssetCode = ac
		it.Strategy = strings.ToLower(strings.TrimSpace(it.Strategy))
		it.UpdatedBy = updatedBy
		if err := tx.Create(it).Error; err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit().Error
}

func (r *currencyGlobalTransferAuditRuleRepo) ReplaceAll(ctx context.Context, updatedBy int64, rulesByAsset map[string][]*model.CurrencyGlobalTransferAuditRuleModel) error {
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

	// Soft-delete all existing active rules
	if err := tx.
		Where("deleted_at IS NULL").
		Delete(&model.CurrencyGlobalTransferAuditRuleModel{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	for assetCode, rules := range rulesByAsset {
		ac := strings.ToUpper(strings.TrimSpace(assetCode))
		if ac == "" {
			continue
		}
		for _, it := range rules {
			if it == nil {
				continue
			}
			it.AssetCode = ac
			it.Strategy = strings.ToLower(strings.TrimSpace(it.Strategy))
			it.UpdatedBy = updatedBy
			if err := tx.Create(it).Error; err != nil {
				tx.Rollback()
				return err
			}
		}
	}
	return tx.Commit().Error
}
