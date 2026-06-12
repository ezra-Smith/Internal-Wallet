package model

import (
	commonModel "internalwallet/common/model"
	"time"
)

type TrustedDeviceModel struct {
	commonModel.BaseModel
	UserId             int64     `gorm:"column:user_id;type:bigint;not null;index;uniqueIndex:uniq_user_device"`
	DeviceId           string    `gorm:"column:device_id;type:varchar(100);default:'';uniqueIndex:uniq_user_device"`
	DeviceName         string    `gorm:"column:device_name;type:varchar(100);default:''"`
	DeviceModel        string    `gorm:"column:device_model;type:varchar(50);default:''"`
	Platform           string    `gorm:"column:platform;type:varchar(20);default:''"`
	OsVersion          string    `gorm:"column:os_version;type:varchar(20);default:''"`
	BiometricEnabled   bool      `gorm:"column:biometric_enabled;type:tinyint(1);default:0"`
	BiometricType      int32     `gorm:"column:biometric_type;type:tinyint;default:0"`
	BiometricPublicKey string    `gorm:"column:biometric_public_key;type:text"`
	LastUsedAt         time.Time `gorm:"column:last_used_at;type:datetime"`
	LastIp             string    `gorm:"column:last_ip;type:varchar(50);default:''"`
	Status             int32     `gorm:"column:status;type:tinyint;default:1;index"`
	TrustedAt          time.Time `gorm:"column:trusted_at;type:datetime"`
	RemovedAt          time.Time `gorm:"column:removed_at;type:datetime"`
	CreatedAt          time.Time `gorm:"column:created_at;type:datetime"`
	UpdatedAt          time.Time `gorm:"column:updated_at;type:datetime"`
}

func (TrustedDeviceModel) TableName() string { return "trusted_devices" }
