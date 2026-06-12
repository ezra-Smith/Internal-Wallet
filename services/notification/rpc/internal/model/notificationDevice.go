package model

import (
	"internalwallet/common/model"
	"time"
)

// NotificationDevice 推送设备模型（对应 notification_devices 表）
type NotificationDevice struct {
	model.BaseModel

	UserID         int64     `gorm:"column:user_id;not null;index:idx_notification_devices_user_id" json:"user_id"`
	RegistrationID string    `gorm:"column:registration_id;size:100;not null;uniqueIndex:uk_notification_devices_registration_id" json:"registration_id"`
	Platform       string    `gorm:"column:platform;size:20;not null" json:"platform"` // ios/android
	DeviceModel    string    `gorm:"column:device_model;size:100;not null;default:''" json:"device_model"`
	OSVersion      string    `gorm:"column:os_version;size:50;not null;default:''" json:"os_version"`
	AppVersion     string    `gorm:"column:app_version;size:50;not null;default:''" json:"app_version"`
	IsActive       bool      `gorm:"column:is_active;not null;default:true;index:idx_notification_devices_is_active" json:"is_active"`
	LastActiveAt   time.Time `gorm:"column:last_active_at;index:idx_notification_devices_last_active_at" json:"last_active_at"`
	DeviceToken    string    `gorm:"column:device_token;size:255;not null;default:''" json:"device_token"`
}

// TableName 指定表名
func (NotificationDevice) TableName() string {
	return "notification_devices"
}

// IsIOS 判断是否为iOS设备
func (d *NotificationDevice) IsIOS() bool {
	return d.Platform == "ios"
}

// IsAndroid 判断是否为Android设备
func (d *NotificationDevice) IsAndroid() bool {
	return d.Platform == "android"
}
