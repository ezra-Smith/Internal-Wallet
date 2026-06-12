package repository

import (
	"context"
	"strings"

	commonRepo "internalwallet/common/repository"
	"internalwallet/services/business/rpc/internal/model"

	"gorm.io/gorm"
)

type CurrencyWithdrawAuditRuleRepository interface {
	commonRepo.BaseRepository[model.CurrencyWithdrawAuditRuleModel]
	ListByAssetChain(ctx context.Context, assetCode, chainCode string) ([]model.CurrencyWithdrawAuditRuleModel, error)
}

type currencyWithdrawAuditRuleRepo struct {
	commonRepo.BaseRepository[model.CurrencyWithdrawAuditRuleModel]
}

func NewCurrencyWithdrawAuditRuleRepository(db *gorm.DB) CurrencyWithdrawAuditRuleRepository {
	return &currencyWithdrawAuditRuleRepo{
		BaseRepository: commonRepo.NewBaseRepository[model.CurrencyWithdrawAuditRuleModel](db),
	}
}

func (r *currencyWithdrawAuditRuleRepo) ListByAssetChain(ctx context.Context, assetCode, chainCode string) ([]model.CurrencyWithdrawAuditRuleModel, error) {
	assetCode = strings.ToUpper(strings.TrimSpace(assetCode))
	chainCode = strings.ToUpper(strings.TrimSpace(chainCode))
	if assetCode == "" || chainCode == "" {
		return []model.CurrencyWithdrawAuditRuleModel{}, nil
	}
	var rows []model.CurrencyWithdrawAuditRuleModel
	err := r.GetDB().WithContext(ctx).
		Where("asset_code = ? AND chain_code = ? AND enabled = 1", assetCode, chainCode).
		Order("sort_order ASC, id ASC").
		Find(&rows).Error
	return rows, err
}
