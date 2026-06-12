package repository

import (
	"context"
	commonRepo "internalwallet/common/repository"
	"internalwallet/services/notification/rpc/internal/model"

	"gorm.io/gorm"
)

// SettingsRepository 用户设置仓储接口
// 嵌入 BaseRepository，继承基础 CRUD 方法
type SettingsRepository interface {
	commonRepo.BaseRepository[model.NotificationUserSettings]

	// 业务特定方法
	FindByUserID(ctx context.Context, userID int64) (*model.NotificationUserSettings, error)
	FindOrCreateByUserID(ctx context.Context, userID int64) (*model.NotificationUserSettings, error)
	DeleteByUserID(ctx context.Context, userID int64) error
}

type settingsRepository struct {
	commonRepo.BaseRepository[model.NotificationUserSettings]
}

// NewSettingsRepository 创建用户设置仓储实例
func NewSettingsRepository(db *gorm.DB) SettingsRepository {
	return &settingsRepository{
		BaseRepository: commonRepo.NewBaseRepository[model.NotificationUserSettings](db),
	}
}

// FindByUserID 根据用户ID查询设置
func (r *settingsRepository) FindByUserID(ctx context.Context, userID int64) (*model.NotificationUserSettings, error) {
	var settings model.NotificationUserSettings
	err := r.GetDB().WithContext(ctx).Where("user_id = ?", userID).First(&settings).Error
	if err != nil {
		return nil, err
	}
	return &settings, nil
}

// FindOrCreateByUserID 查询或创建用户设置（如果不存在则创建默认设置）
func (r *settingsRepository) FindOrCreateByUserID(ctx context.Context, userID int64) (*model.NotificationUserSettings, error) {
	settings, err := r.FindByUserID(ctx, userID)
	if err == nil {
		return settings, nil
	}

	if err != gorm.ErrRecordNotFound {
		return nil, err
	}

	// 创建默认设置
	defaultSettings := &model.NotificationUserSettings{
		UserID:            userID,
		EnableTransaction: true,
		EnableSecurity:    true,
		EnableSystem:      true,
		EnablePriceAlert:  true,
		Language:          "zh-CN",
	}

	if err := r.Create(ctx, defaultSettings); err != nil {
		return nil, err
	}

	return defaultSettings, nil
}

// DeleteByUserID 删除用户设置（软删除）
func (r *settingsRepository) DeleteByUserID(ctx context.Context, userID int64) error {
	return r.GetDB().WithContext(ctx).
		Where("user_id = ?", userID).
		Delete(&model.NotificationUserSettings{}).
		Error
}
