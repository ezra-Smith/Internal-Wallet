package repository

import (
	"context"
	"strings"

	"internalwallet/services/business/rpc/internal/model"

	"gorm.io/gorm"
)

type Web3UserSettingsRepository interface {
	WithTx(tx *gorm.DB) Web3UserSettingsRepository
	GetDB() *gorm.DB

	// 根据用户ID查询设置
	FindByUserID(ctx context.Context, userID int64) (*model.Web3UserSettingsModel, error)
	// 根据设备ID查询设置
	FindByDeviceID(ctx context.Context, deviceID string) (*model.Web3UserSettingsModel, error)
	// 创建或更新设置（Upsert）
	Upsert(ctx context.Context, settings *model.Web3UserSettingsModel) error
	// 更新部分字段
	UpdateFields(ctx context.Context, userID int64, fields map[string]interface{}) error
}

type web3UserSettingsRepo struct {
	db *gorm.DB
}

func NewWeb3UserSettingsRepository(db *gorm.DB) Web3UserSettingsRepository {
	return &web3UserSettingsRepo{db: db}
}

func (r *web3UserSettingsRepo) WithTx(tx *gorm.DB) Web3UserSettingsRepository {
	return &web3UserSettingsRepo{db: tx}
}

func (r *web3UserSettingsRepo) GetDB() *gorm.DB {
	return r.db
}

func (r *web3UserSettingsRepo) FindByUserID(ctx context.Context, userID int64) (*model.Web3UserSettingsModel, error) {
	if userID <= 0 {
		return nil, gorm.ErrRecordNotFound
	}

	var settings model.Web3UserSettingsModel
	err := r.db.WithContext(ctx).
		Where("web3_user_id = ?", userID).
		First(&settings).Error

	if err != nil {
		return nil, err
	}
	return &settings, nil
}

func (r *web3UserSettingsRepo) FindByDeviceID(ctx context.Context, deviceID string) (*model.Web3UserSettingsModel, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil, gorm.ErrRecordNotFound
	}

	var settings model.Web3UserSettingsModel
	err := r.db.WithContext(ctx).
		Where("device_id = ?", deviceID).
		First(&settings).Error

	if err != nil {
		return nil, err
	}
	return &settings, nil
}

func (r *web3UserSettingsRepo) Upsert(ctx context.Context, settings *model.Web3UserSettingsModel) error {
	// 使用 GORM 的 Save 方法，如果记录存在则更新，不存在则插入
	return r.db.WithContext(ctx).Save(settings).Error
}

func (r *web3UserSettingsRepo) UpdateFields(ctx context.Context, userID int64, fields map[string]interface{}) error {
	if userID <= 0 {
		return gorm.ErrRecordNotFound
	}

	return r.db.WithContext(ctx).
		Model(&model.Web3UserSettingsModel{}).
		Where("web3_user_id = ?", userID).
		Updates(fields).Error
}
