package repository

import (
	"context"
	"strings"

	commonRepo "internalwallet/common/repository"
	"internalwallet/services/business/rpc/internal/model"

	"gorm.io/gorm"
)

type CurrencyGlobalWithdrawAuditRuleRepository interface {
	commonRepo.BaseRepository[model.CurrencyGlobalWithdrawAuditRuleModel]
	ListByChain(ctx context.Context, chainCode string) ([]model.CurrencyGlobalWithdrawAuditRuleModel, error)
}

type currencyGlobalWithdrawAuditRuleRepo struct {
	commonRepo.BaseRepository[model.CurrencyGlobalWithdrawAuditRuleModel]
}

func NewCurrencyGlobalWithdrawAuditRuleRepository(db *gorm.DB) CurrencyGlobalWithdrawAuditRuleRepository {
	return &currencyGlobalWithdrawAuditRuleRepo{
		BaseRepository: commonRepo.NewBaseRepository[model.CurrencyGlobalWithdrawAuditRuleModel](db),
	}
}

func (r *currencyGlobalWithdrawAuditRuleRepo) ListByChain(ctx context.Context, chainCode string) ([]model.CurrencyGlobalWithdrawAuditRuleModel, error) {
	chainCode = strings.ToUpper(strings.TrimSpace(chainCode))
	if chainCode == "" {
		return []model.CurrencyGlobalWithdrawAuditRuleModel{}, nil
	}
	var rows []model.CurrencyGlobalWithdrawAuditRuleModel
	err := r.GetDB().WithContext(ctx).
		Where("chain_code = ? AND enabled = 1", chainCode).
		Order("sort_order ASC, id ASC").
		Find(&rows).Error
	return rows, err
}
