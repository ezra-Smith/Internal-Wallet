package repository

import (
	"context"

	commonRepo "internalwallet/common/repository"
	"internalwallet/services/admin/rpc/internal/model"

	"gorm.io/gorm"
)

type AlertConfigRepository interface {
	commonRepo.BaseRepository[model.AlertConfigModel]

	// 自定义方法
	FindByType(ctx context.Context, alertType string) ([]*model.AlertConfigModel, error)
	FindEnabled(ctx context.Context) ([]*model.AlertConfigModel, error)
	FindByEnabledAndType(ctx context.Context, alertType string) ([]*model.AlertConfigModel, error)
	FindByName(ctx context.Context, name string) (*model.AlertConfigModel, error)
	UpdateEnabled(ctx context.Context, id int64, enabled bool, updatedBy int64) error
}

type alertConfigRepo struct {
	commonRepo.BaseRepository[model.AlertConfigModel]
}

func NewAlertConfigRepository(db *gorm.DB) AlertConfigRepository {
	return &alertConfigRepo{
		BaseRepository: commonRepo.NewBaseRepository[model.AlertConfigModel](db),
	}
}

// FindByType 根据类型查询
func (r *alertConfigRepo) FindByType(ctx context.Context, alertType string) ([]*model.AlertConfigModel, error) {
	var items []*model.AlertConfigModel
	err := r.GetDB().WithContext(ctx).
		Where("alert_type = ?", alertType).
		Order("id ASC").
		Find(&items).Error
	return items, err
}

// FindEnabled 查询所有启用的配置
func (r *alertConfigRepo) FindEnabled(ctx context.Context) ([]*model.AlertConfigModel, error) {
	var items []*model.AlertConfigModel
	err := r.GetDB().WithContext(ctx).
		Where("enabled = ?", true).
		Order("id ASC").
		Find(&items).Error
	return items, err
}

// FindByEnabledAndType 查询启用的特定类型配置
func (r *alertConfigRepo) FindByEnabledAndType(ctx context.Context, alertType string) ([]*model.AlertConfigModel, error) {
	var items []*model.AlertConfigModel
	err := r.GetDB().WithContext(ctx).
		Where("alert_type = ? AND enabled = ?", alertType, true).
		Order("id ASC").
		Find(&items).Error
	return items, err
}

// FindByName 根据名称查找（避免重名）
func (r *alertConfigRepo) FindByName(ctx context.Context, name string) (*model.AlertConfigModel, error) {
	var item model.AlertConfigModel
	err := r.GetDB().WithContext(ctx).
		Where("name = ?", name).
		First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// UpdateEnabled 更新启用状态
func (r *alertConfigRepo) UpdateEnabled(ctx context.Context, id int64, enabled bool, updatedBy int64) error {
	return r.GetDB().WithContext(ctx).
		Model(&model.AlertConfigModel{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"enabled":    enabled,
			"updated_by": updatedBy,
		}).Error
}
