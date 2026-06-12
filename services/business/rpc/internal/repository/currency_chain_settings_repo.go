package repository

import (
	"context"
	"strings"

	commonRepo "internalwallet/common/repository"
	"internalwallet/services/business/rpc/internal/model"

	"gorm.io/gorm"
)

type CurrencyChainSettingsRepository interface {
	commonRepo.BaseRepository[model.CurrencyChainSettingsModel]
	ListByAssetCode(ctx context.Context, assetCode string) ([]model.CurrencyChainSettingsModel, error)
	FindByAssetChain(ctx context.Context, assetCode, chainCode string) (*model.CurrencyChainSettingsModel, error)
	ListAllEnabled(ctx context.Context) ([]model.CurrencyChainSettingsModel, error)
	ListWithdrawEnabledAssetsByChain(ctx context.Context, chainCode string) ([]string, error)
	// ListWeb3SupportedAssetsByChain returns assets that are enabled for Web3 display on the given chain
	ListWeb3SupportedAssetsByChain(ctx context.Context, chainCode string) ([]model.CurrencyChainSettingsModel, error)
}

type currencyChainSettingsRepo struct {
	commonRepo.BaseRepository[model.CurrencyChainSettingsModel]
}

func NewCurrencyChainSettingsRepository(db *gorm.DB) CurrencyChainSettingsRepository {
	return &currencyChainSettingsRepo{BaseRepository: commonRepo.NewBaseRepository[model.CurrencyChainSettingsModel](db)}
}

func (r *currencyChainSettingsRepo) ListByAssetCode(ctx context.Context, assetCode string) ([]model.CurrencyChainSettingsModel, error) {
	assetCode = strings.ToUpper(strings.TrimSpace(assetCode))
	if assetCode == "" {
		return []model.CurrencyChainSettingsModel{}, nil
	}
	var rows []model.CurrencyChainSettingsModel
	err := r.GetDB().WithContext(ctx).
		Where("asset_code = ?", assetCode).
		Order("chain_code ASC").
		Find(&rows).Error
	return rows, err
}

func (r *currencyChainSettingsRepo) FindByAssetChain(ctx context.Context, assetCode, chainCode string) (*model.CurrencyChainSettingsModel, error) {
	assetCode = strings.ToUpper(strings.TrimSpace(assetCode))
	chainCode = strings.ToUpper(strings.TrimSpace(chainCode))
	if assetCode == "" || chainCode == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var m model.CurrencyChainSettingsModel
	err := r.GetDB().WithContext(ctx).
		Where("asset_code = ? AND chain_code = ?", assetCode, chainCode).
		First(&m).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// ListAllEnabled returns all enabled currency-chain mappings (status=1, not deleted).
func (r *currencyChainSettingsRepo) ListAllEnabled(ctx context.Context) ([]model.CurrencyChainSettingsModel, error) {
	var rows []model.CurrencyChainSettingsModel
	err := r.GetDB().WithContext(ctx).
		Where("status = ? AND deleted_at IS NULL", 1).
		Order("asset_code ASC, chain_code ASC").
		Find(&rows).Error
	return rows, err
}

// ListWithdrawEnabledAssetsByChain returns distinct asset_codes that are withdraw-enabled on the given chain.
// Source of truth: currency_chain_settings (status=1, deleted_at IS NULL, withdraw_enabled=1).
func (r *currencyChainSettingsRepo) ListWithdrawEnabledAssetsByChain(ctx context.Context, chainCode string) ([]string, error) {
	chainCode = strings.ToUpper(strings.TrimSpace(chainCode))
	if chainCode == "" {
		return []string{}, nil
	}

	var raw []string
	err := r.GetDB().WithContext(ctx).
		Model(&model.CurrencyChainSettingsModel{}).
		Distinct("asset_code").
		Where("chain_code = ? AND status = ? AND deleted_at IS NULL AND withdraw_enabled = 1 AND asset_code <> ''", chainCode, 1).
		Pluck("asset_code", &raw).Error
	if err != nil {
		return nil, err
	}

	out := make([]string, 0, len(raw))
	seen := map[string]struct{}{}
	for _, c := range raw {
		cc := strings.ToUpper(strings.TrimSpace(c))
		if cc == "" {
			continue
		}
		if _, ok := seen[cc]; ok {
			continue
		}
		seen[cc] = struct{}{}
		out = append(out, cc)
	}
	return out, nil
}

// ListWeb3SupportedAssetsByChain returns all assets that are enabled for Web3 display on the given chain.
// Filters: status=1, deleted_at IS NULL, web3_asset_display_enabled=1
func (r *currencyChainSettingsRepo) ListWeb3SupportedAssetsByChain(ctx context.Context, chainCode string) ([]model.CurrencyChainSettingsModel, error) {
	chainCode = strings.ToUpper(strings.TrimSpace(chainCode))
	if chainCode == "" {
		return []model.CurrencyChainSettingsModel{}, nil
	}

	var rows []model.CurrencyChainSettingsModel
	err := r.GetDB().WithContext(ctx).
		Where("chain_code = ? AND status = ? AND deleted_at IS NULL AND web3_asset_display_enabled = 1", chainCode, 1).
		Order("asset_code ASC").
		Find(&rows).Error
	return rows, err
}
