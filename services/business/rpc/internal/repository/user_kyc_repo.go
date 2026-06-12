package repository

import (
	"context"
	commonRepo "internalwallet/common/repository"
	"internalwallet/services/business/rpc/internal/model"

	"gorm.io/gorm"
)

type UserKycRepository interface {
	commonRepo.BaseRepository[model.UserKycModel]
	GetKycByUserID(ctx context.Context, userID int64) (*model.UserKycModel, error)
	UpsertKyc(ctx context.Context, k *model.UserKycModel) error
}

type userKycRepo struct {
	commonRepo.BaseRepository[model.UserKycModel]
}

func NewUserKycRepository(db *gorm.DB) UserKycRepository {
	return &userKycRepo{
		BaseRepository: commonRepo.NewBaseRepository[model.UserKycModel](db),
	}
}

func (r *userKycRepo) GetKycByUserID(ctx context.Context, userID int64) (*model.UserKycModel, error) {
	var m model.UserKycModel
	err := r.GetDB().WithContext(context.Background()).Where("user_id = ?", userID).First(&m).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *userKycRepo) UpsertKyc(ctx context.Context, k *model.UserKycModel) error {
	var count int64
	if err := r.GetDB().WithContext(context.Background()).Model(&model.UserKycModel{}).Where("user_id = ?", k.UserId).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return r.GetDB().WithContext(context.Background()).Create(k).Error
	}
	return r.GetDB().WithContext(context.Background()).Model(&model.UserKycModel{}).Where("user_id = ?", k.UserId).Updates(map[string]interface{}{
		"real_name": k.RealName,
		"id_number": k.IdNumber,
		"birthday":  k.Birthday,
		"gender":    k.Gender,
		"address":   k.Address,
	}).Error
}
