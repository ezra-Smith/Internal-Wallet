package repository

import (
	"context"
	commonRepo "internalwallet/common/repository"
	"internalwallet/services/business/rpc/internal/model"

	"gorm.io/gorm"
)

type LanguageRepository interface {
	commonRepo.BaseRepository[model.LanguageModel]
	ListEnabledLanguages(ctx context.Context) ([]model.LanguageModel, error)
}

type languageRepo struct {
	commonRepo.BaseRepository[model.LanguageModel]
}

func NewLanguageRepository(db *gorm.DB) LanguageRepository {
	return &languageRepo{
		BaseRepository: commonRepo.NewBaseRepository[model.LanguageModel](db),
	}
}

func (r *languageRepo) ListEnabledLanguages(ctx context.Context) ([]model.LanguageModel, error) {
	var rows []model.LanguageModel
	err := r.GetDB().WithContext(context.Background()).Model(&model.LanguageModel{}).Where("status = 1").Order("id ASC").Find(&rows).Error
	return rows, err
}
