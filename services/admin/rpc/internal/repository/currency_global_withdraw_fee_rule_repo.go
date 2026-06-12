package repository

import (
	"context"
	"fmt"
	"strings"

	"internalwallet/services/admin/rpc/internal/model"

	"gorm.io/gorm"
)

type CurrencyGlobalWithdrawFeeRuleRepository interface {
	WithTx(tx *gorm.DB) CurrencyGlobalWithdrawFeeRuleRepository
	GetDB() *gorm.DB

	ListByChain(ctx context.Context, chainCode string) ([]*model.CurrencyGlobalWithdrawFeeRuleModel, error)
	ReplaceForChain(ctx context.Context, chainCode string, updatedBy int64, rules []*model.CurrencyGlobalWithdrawFeeRuleModel) error
}

type currencyGlobalWithdrawFeeRuleRepo struct{ db *gorm.DB }

func NewCurrencyGlobalWithdrawFeeRuleRepository(db *gorm.DB) CurrencyGlobalWithdrawFeeRuleRepository {
	return &currencyGlobalWithdrawFeeRuleRepo{db: db}
}
func (r *currencyGlobalWithdrawFeeRuleRepo) WithTx(tx *gorm.DB) CurrencyGlobalWithdrawFeeRuleRepository {
	return &currencyGlobalWithdrawFeeRuleRepo{db: tx}
}
func (r *currencyGlobalWithdrawFeeRuleRepo) GetDB() *gorm.DB { return r.db }

func (r *currencyGlobalWithdrawFeeRuleRepo) ListByChain(ctx context.Context, chainCode string) ([]*model.CurrencyGlobalWithdrawFeeRuleModel, error) {
	chainCode = strings.ToUpper(strings.TrimSpace(chainCode))
	if chainCode == "" {
		return []*model.CurrencyGlobalWithdrawFeeRuleModel{}, nil
	}
	var items []*model.CurrencyGlobalWithdrawFeeRuleModel
	err := r.db.WithContext(ctx).
		Where("chain_code = ?", chainCode).
		Order("sort_order ASC, id ASC").
		Find(&items).Error
	return items, err
}

func (r *currencyGlobalWithdrawFeeRuleRepo) ReplaceForChain(ctx context.Context, chainCode string, updatedBy int64, rules []*model.CurrencyGlobalWithdrawFeeRuleModel) error {
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
		Delete(&model.CurrencyGlobalWithdrawFeeRuleModel{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	for _, it := range rules {
		if it == nil {
			continue
		}
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
