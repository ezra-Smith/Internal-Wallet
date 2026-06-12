package repository

import (
	"context"
	"strings"

	commonRepo "internalwallet/common/repository"
	"internalwallet/services/business/rpc/internal/model"

	"gorm.io/gorm"
)

type CurrencyGlobalWithdrawFeeRuleRepository interface {
	commonRepo.BaseRepository[model.CurrencyGlobalWithdrawFeeRuleModel]
	ListByChain(ctx context.Context, chainCode string) ([]model.CurrencyGlobalWithdrawFeeRuleModel, error)
	ListByAsset(ctx context.Context, assetCode string) ([]model.CurrencyGlobalWithdrawFeeRuleModel, error)
}

type currencyGlobalWithdrawFeeRuleRepo struct {
	commonRepo.BaseRepository[model.CurrencyGlobalWithdrawFeeRuleModel]
}

func NewCurrencyGlobalWithdrawFeeRuleRepository(db *gorm.DB) CurrencyGlobalWithdrawFeeRuleRepository {
	return &currencyGlobalWithdrawFeeRuleRepo{BaseRepository: commonRepo.NewBaseRepository[model.CurrencyGlobalWithdrawFeeRuleModel](db)}
}

func (r *currencyGlobalWithdrawFeeRuleRepo) ListByChain(ctx context.Context, chainCode string) ([]model.CurrencyGlobalWithdrawFeeRuleModel, error) {
	chainCode = strings.ToUpper(strings.TrimSpace(chainCode))
	if chainCode == "" {
		return []model.CurrencyGlobalWithdrawFeeRuleModel{}, nil
	}
	var rows []model.CurrencyGlobalWithdrawFeeRuleModel
	err := r.GetDB().WithContext(ctx).
		Where("chain_code = ? AND enabled = 1", chainCode).
		Order("sort_order ASC, id ASC").
		Find(&rows).Error
	return rows, err
}

func (r *currencyGlobalWithdrawFeeRuleRepo) ListByAsset(ctx context.Context, assetCode string) ([]model.CurrencyGlobalWithdrawFeeRuleModel, error) {
	assetCode = strings.ToUpper(strings.TrimSpace(assetCode))
	if assetCode == "" {
		return []model.CurrencyGlobalWithdrawFeeRuleModel{}, nil
	}
	var rows []model.CurrencyGlobalWithdrawFeeRuleModel
	err := r.GetDB().WithContext(ctx).
		Where("chain_code = ? AND enabled = 1", assetCode).
		Order("sort_order ASC, id ASC").
		Find(&rows).Error
	return rows, err
}
