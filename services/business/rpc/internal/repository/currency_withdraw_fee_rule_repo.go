package repository

import (
	"context"
	"strings"

	commonRepo "internalwallet/common/repository"
	"internalwallet/services/business/rpc/internal/model"

	"gorm.io/gorm"
)

type CurrencyWithdrawFeeRuleRepository interface {
	commonRepo.BaseRepository[model.CurrencyWithdrawFeeRuleModel]
	ListByAssetChain(ctx context.Context, assetCode, chainCode string) ([]model.CurrencyWithdrawFeeRuleModel, error)
}

type currencyWithdrawFeeRuleRepo struct {
	commonRepo.BaseRepository[model.CurrencyWithdrawFeeRuleModel]
}

func NewCurrencyWithdrawFeeRuleRepository(db *gorm.DB) CurrencyWithdrawFeeRuleRepository {
	return &currencyWithdrawFeeRuleRepo{BaseRepository: commonRepo.NewBaseRepository[model.CurrencyWithdrawFeeRuleModel](db)}
}

func (r *currencyWithdrawFeeRuleRepo) ListByAssetChain(ctx context.Context, assetCode, chainCode string) ([]model.CurrencyWithdrawFeeRuleModel, error) {
	assetCode = strings.ToUpper(strings.TrimSpace(assetCode))
	chainCode = strings.ToUpper(strings.TrimSpace(chainCode))
	if assetCode == "" || chainCode == "" {
		return []model.CurrencyWithdrawFeeRuleModel{}, nil
	}
	var rows []model.CurrencyWithdrawFeeRuleModel
	err := r.GetDB().WithContext(ctx).
		Where("asset_code = ? AND chain_code = ? AND enabled = 1", assetCode, chainCode).
		Order("sort_order ASC, id ASC").
		Find(&rows).Error
	return rows, err
}
