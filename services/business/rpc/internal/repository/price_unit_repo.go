package repository

import (
	"context"
	"gorm.io/gorm"
	commonRepo "internalwallet/common/repository"
	"internalwallet/services/business/rpc/internal/model"
)

type PriceUnitRepository interface {
	commonRepo.BaseRepository[model.PriceUnitModel]
	ListEnabledUnits(ctx context.Context) ([]model.PriceUnitModel, error)
}

type priceUnitRepo struct {
	commonRepo.BaseRepository[model.PriceUnitModel]
}

func NewPriceUnitRepository(db *gorm.DB) PriceUnitRepository {
	return &priceUnitRepo{
		BaseRepository: commonRepo.NewBaseRepository[model.PriceUnitModel](db),
	}
}

func (r *priceUnitRepo) ListEnabledUnits(ctx context.Context) ([]model.PriceUnitModel, error) {
	var rows []model.PriceUnitModel
	err := r.GetDB().WithContext(context.Background()).Model(&model.PriceUnitModel{}).Where("status = 1").Order("id ASC").Find(&rows).Error
	return rows, err
}
