package repository

import (
	"context"

	commonRepo "internalwallet/common/repository"
	"internalwallet/services/admin/rpc/internal/model"

	"gorm.io/gorm"
)

type AdminSystemConfigRepository interface {
	commonRepo.BaseRepository[model.AdminSystemConfigModel]

	GetByCategory(ctx context.Context, category string) ([]*model.AdminSystemConfigModel, error)
	GetOne(ctx context.Context, category, key string) (*model.AdminSystemConfigModel, error)
	Upsert(ctx context.Context, category, key string, value []byte, updatedBy int64) (*model.AdminSystemConfigModel, error)
}

type adminSystemConfigRepo struct {
	commonRepo.BaseRepository[model.AdminSystemConfigModel]
}

func NewAdminSystemConfigRepository(db *gorm.DB) AdminSystemConfigRepository {
	return &adminSystemConfigRepo{
		BaseRepository: commonRepo.NewBaseRepository[model.AdminSystemConfigModel](db),
	}
}

func (r *adminSystemConfigRepo) GetByCategory(ctx context.Context, category string) ([]*model.AdminSystemConfigModel, error) {
	var items []*model.AdminSystemConfigModel
	err := r.GetDB().WithContext(ctx).
		Where("category = ?", category).
		Order("id ASC").
		Find(&items).Error
	return items, err
}

func (r *adminSystemConfigRepo) GetOne(ctx context.Context, category, key string) (*model.AdminSystemConfigModel, error) {
	var item model.AdminSystemConfigModel
	err := r.GetDB().WithContext(ctx).
		Where("category = ? AND key_name = ?", category, key).
		First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *adminSystemConfigRepo) Upsert(ctx context.Context, category, key string, value []byte, updatedBy int64) (*model.AdminSystemConfigModel, error) {
	var existing model.AdminSystemConfigModel
	err := r.GetDB().WithContext(ctx).
		Where("category = ? AND key_name = ?", category, key).
		First(&existing).Error

	if err == nil {
		existing.Value = value
		existing.UpdatedBy = updatedBy
		if updateErr := r.GetDB().WithContext(ctx).Save(&existing).Error; updateErr != nil {
			return nil, updateErr
		}
		return &existing, nil
	}
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}

	m := &model.AdminSystemConfigModel{
		Category:  category,
		KeyName:   key,
		Value:     value,
		UpdatedBy: updatedBy,
	}
	if createErr := r.GetDB().WithContext(ctx).Create(m).Error; createErr != nil {
		return nil, createErr
	}
	return m, nil
}
