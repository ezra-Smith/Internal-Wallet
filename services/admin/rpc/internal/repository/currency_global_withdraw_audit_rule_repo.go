package repository

import (
	"context"
	"fmt"
	"strings"

	"internalwallet/services/admin/rpc/internal/model"

	"gorm.io/gorm"
)

type CurrencyGlobalWithdrawAuditRuleRepository interface {
	WithTx(tx *gorm.DB) CurrencyGlobalWithdrawAuditRuleRepository
	GetDB() *gorm.DB

	ListByChain(ctx context.Context, chainCode string) ([]*model.CurrencyGlobalWithdrawAuditRuleModel, error)
	ReplaceForChain(ctx context.Context, chainCode string, updatedBy int64, rules []*model.CurrencyGlobalWithdrawAuditRuleModel) error
}

type currencyGlobalWithdrawAuditRuleRepo struct{ db *gorm.DB }

func NewCurrencyGlobalWithdrawAuditRuleRepository(db *gorm.DB) CurrencyGlobalWithdrawAuditRuleRepository {
	return &currencyGlobalWithdrawAuditRuleRepo{db: db}
}
func (r *currencyGlobalWithdrawAuditRuleRepo) WithTx(tx *gorm.DB) CurrencyGlobalWithdrawAuditRuleRepository {
	return &currencyGlobalWithdrawAuditRuleRepo{db: tx}
}
func (r *currencyGlobalWithdrawAuditRuleRepo) GetDB() *gorm.DB { return r.db }

func (r *currencyGlobalWithdrawAuditRuleRepo) ListByChain(ctx context.Context, chainCode string) ([]*model.CurrencyGlobalWithdrawAuditRuleModel, error) {
	chainCode = strings.ToUpper(strings.TrimSpace(chainCode))
	if chainCode == "" {
		return []*model.CurrencyGlobalWithdrawAuditRuleModel{}, nil
	}
	var items []*model.CurrencyGlobalWithdrawAuditRuleModel
	err := r.db.WithContext(ctx).
		Where("chain_code = ?", chainCode).
		Order("sort_order ASC, id ASC").
		Find(&items).Error
	return items, err
}

func (r *currencyGlobalWithdrawAuditRuleRepo) ReplaceForChain(ctx context.Context, chainCode string, updatedBy int64, rules []*model.CurrencyGlobalWithdrawAuditRuleModel) error {
	chainCode = strings.ToUpper(strings.TrimSpace(chainCode))
	if chainCode == "" {
		return fmt.Errorf("invalid chain")
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
		Where("chain_code = ?", chainCode).
		Delete(&model.CurrencyGlobalWithdrawAuditRuleModel{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	for _, it := range rules {
		if it == nil {
			continue
		}
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
