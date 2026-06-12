package repository

import (
	"context"
	"errors"
	"fmt"

	commonRepo "internalwallet/common/repository"
	"internalwallet/services/business/rpc/internal/model"

	"gorm.io/gorm"
)

type UserWhitelistSettingsRepository interface {
	commonRepo.BaseRepository[model.UserWhitelistSettingsModel]

	GetByUserID(ctx context.Context, userID int64) (*model.UserWhitelistSettingsModel, error)
}

type userWhitelistSettingsRepo struct {
	commonRepo.BaseRepository[model.UserWhitelistSettingsModel]
}

func NewUserWhitelistSettingsRepository(db *gorm.DB) UserWhitelistSettingsRepository {
	return &userWhitelistSettingsRepo{
		BaseRepository: commonRepo.NewBaseRepository[model.UserWhitelistSettingsModel](db),
	}
}

func (r *userWhitelistSettingsRepo) GetByUserID(ctx context.Context, userID int64) (*model.UserWhitelistSettingsModel, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("invalid user_id")
	}
	var s model.UserWhitelistSettingsModel
	err := r.GetDB().WithContext(ctx).
		Model(&model.UserWhitelistSettingsModel{}).
		Where("user_id = ?", userID).
		First(&s).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &s, err
}
