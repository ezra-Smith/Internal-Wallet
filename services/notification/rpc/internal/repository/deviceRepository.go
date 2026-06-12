package repository

import (
	"context"
	commonRepo "internalwallet/common/repository"
	"internalwallet/services/notification/rpc/internal/model"
	"time"

	"gorm.io/gorm"
)

// DeviceRepository 设备仓储接口
// 嵌入 BaseRepository，继承基础 CRUD 方法
type DeviceRepository interface {
	commonRepo.BaseRepository[model.NotificationDevice]

	FindByRegistrationID(ctx context.Context, registrationID string) (*model.NotificationDevice, error)
	FindByDeviceToken(ctx context.Context, deviceToken string) (*model.NotificationDevice, error)
	FindByUserID(ctx context.Context, userID int64, activeOnly bool) ([]*model.NotificationDevice, error)
	FindActiveDevicesByUserID(ctx context.Context, userID int64) ([]*model.NotificationDevice, error)
	FindActiveDevicesByUserIDs(ctx context.Context, userIDs []int64) ([]*model.NotificationDevice, error)
	DeactivateDevice(ctx context.Context, registrationID string) error
	DeactivateDeviceByToken(ctx context.Context, deviceToken string) error
	DeactivateDevicesByUserID(ctx context.Context, userID int64) error
	UpdateLastActiveTime(ctx context.Context, registrationID string) error
	UpdateLastActiveTimeByToken(ctx context.Context, deviceToken string) error
}

type deviceRepository struct {
	commonRepo.BaseRepository[model.NotificationDevice]
}

// NewDeviceRepository 创建设备仓储实例
func NewDeviceRepository(db *gorm.DB) DeviceRepository {
	return &deviceRepository{
		BaseRepository: commonRepo.NewBaseRepository[model.NotificationDevice](db),
	}
}

// FindByRegistrationID 根据 Registration ID 查询设备
func (r *deviceRepository) FindByRegistrationID(ctx context.Context, registrationID string) (*model.NotificationDevice, error) {
	var device model.NotificationDevice
	err := r.GetDB().WithContext(ctx).Where("registration_id = ?", registrationID).First(&device).Error
	if err != nil {
		return nil, err
	}
	return &device, nil
}

// FindByDeviceToken 根据设备Token（设备ID）查询设备
func (r *deviceRepository) FindByDeviceToken(ctx context.Context, deviceToken string) (*model.NotificationDevice, error) {
	var device model.NotificationDevice
	err := r.GetDB().WithContext(ctx).Where("device_token = ?", deviceToken).First(&device).Error
	if err != nil {
		return nil, err
	}
	return &device, nil
}

// FindByUserID 根据用户ID查询设备列表
func (r *deviceRepository) FindByUserID(ctx context.Context, userID int64, activeOnly bool) ([]*model.NotificationDevice, error) {
	var devices []*model.NotificationDevice
	query := r.GetDB().WithContext(ctx).Where("user_id = ?", userID)

	if activeOnly {
		query = query.Where("is_active = ?", true)
	}

	err := query.Order("last_active_at DESC").Find(&devices).Error
	return devices, err
}

// FindActiveDevicesByUserID 根据用户ID查询活跃设备
func (r *deviceRepository) FindActiveDevicesByUserID(ctx context.Context, userID int64) ([]*model.NotificationDevice, error) {
	return r.FindByUserID(ctx, userID, true)
}

// FindActiveDevicesByUserIDs 批量查询多个用户的活跃设备
func (r *deviceRepository) FindActiveDevicesByUserIDs(ctx context.Context, userIDs []int64) ([]*model.NotificationDevice, error) {
	var devices []*model.NotificationDevice
	err := r.GetDB().WithContext(ctx).
		Where("user_id IN ?", userIDs).
		Where("is_active = ?", true).
		Order("user_id, last_active_at DESC").
		Find(&devices).Error
	return devices, err
}

// DeactivateDevice 停用设备
func (r *deviceRepository) DeactivateDevice(ctx context.Context, registrationID string) error {
	return r.GetDB().WithContext(ctx).
		Model(&model.NotificationDevice{}).
		Where("registration_id = ?", registrationID).
		Update("is_active", false).
		Error
}

// DeactivateDeviceByToken 通过设备Token停用设备
func (r *deviceRepository) DeactivateDeviceByToken(ctx context.Context, deviceToken string) error {
	return r.GetDB().WithContext(ctx).
		Model(&model.NotificationDevice{}).
		Where("device_token = ?", deviceToken).
		Update("is_active", false).
		Error
}

// DeactivateDevicesByUserID 停用用户的所有设备
func (r *deviceRepository) DeactivateDevicesByUserID(ctx context.Context, userID int64) error {
	return r.GetDB().WithContext(ctx).
		Model(&model.NotificationDevice{}).
		Where("user_id = ?", userID).
		Update("is_active", false).
		Error
}

// UpdateLastActiveTime 更新最后活跃时间
func (r *deviceRepository) UpdateLastActiveTime(ctx context.Context, registrationID string) error {
	return r.GetDB().WithContext(ctx).
		Model(&model.NotificationDevice{}).
		Where("registration_id = ?", registrationID).
		Update("last_active_at", time.Now()).
		Error
}

// UpdateLastActiveTimeByToken 通过设备Token更新最后活跃时间
func (r *deviceRepository) UpdateLastActiveTimeByToken(ctx context.Context, deviceToken string) error {
	return r.GetDB().WithContext(ctx).
		Model(&model.NotificationDevice{}).
		Where("device_token = ?", deviceToken).
		Update("last_active_at", time.Now()).
		Error
}
