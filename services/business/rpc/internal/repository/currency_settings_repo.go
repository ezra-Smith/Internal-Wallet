package repository

import (
	"context"
	"strings"

	commonRepo "internalwallet/common/repository"
	"internalwallet/services/business/rpc/internal/model"

	"gorm.io/gorm"
)

type CurrencySettingsRepository interface {
	commonRepo.BaseRepository[model.CurrencySettingsModel]
	GetByAssetCode(ctx context.Context, assetCode string) (*model.CurrencySettingsModel, error)
	ListByAssetCodes(ctx context.Context, assetCodes []string) ([]model.CurrencySettingsModel, error)
}

type currencySettingsRepo struct {
	commonRepo.BaseRepository[model.CurrencySettingsModel]
}

func NewCurrencySettingsRepository(db *gorm.DB) CurrencySettingsRepository {
	return &currencySettingsRepo{BaseRepository: commonRepo.NewBaseRepository[model.CurrencySettingsModel](db)}
}

func (r *currencySettingsRepo) GetByAssetCode(ctx context.Context, assetCode string) (*model.CurrencySettingsModel, error) {
	assetCode = strings.ToUpper(strings.TrimSpace(assetCode))
	if assetCode == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var m model.CurrencySettingsModel
	err := r.GetDB().WithContext(ctx).
		Where("asset_code = ?", assetCode).
		First(&m).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *currencySettingsRepo) ListByAssetCodes(ctx context.Context, assetCodes []string) ([]model.CurrencySettingsModel, error) {
	codes := make([]string, 0, len(assetCodes))
	for _, c := range assetCodes {
		cc := strings.ToUpper(strings.TrimSpace(c))
		if cc != "" {
			codes = append(codes, cc)
		}
	}
	if len(codes) == 0 {
		return []model.CurrencySettingsModel{}, nil
	}
	var rows []model.CurrencySettingsModel
	err := r.GetDB().WithContext(ctx).
		Where("asset_code IN ?", codes).
		Find(&rows).Error
	return rows, err
}
