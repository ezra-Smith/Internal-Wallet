package model

import "time"

// Web3UserModel maps to web3_users (device-based Web3 wallet users).
type Web3UserModel struct {
	ID       int64  `gorm:"column:id;primaryKey;autoIncrement"`
	DeviceID string `gorm:"column:device_id;type:varchar(128);not null;uniqueIndex"`

	Platform    string  `gorm:"column:platform;type:varchar(20);not null;default:'';index"`
	OsVersion   string  `gorm:"column:os_version;type:varchar(50);not null;default:''"`
	AppVersion  string  `gorm:"column:app_version;type:varchar(50);not null;default:''"`
	DeviceModel string  `gorm:"column:device_model;type:varchar(100);not null;default:''"`
	DeviceName  string  `gorm:"column:device_name;type:varchar(100);not null;default:''"`
	PushToken   *string `gorm:"column:push_token;type:varchar(255)"`
	Locale      *string `gorm:"column:locale;type:varchar(50)"`

	TwoFactorEnabled   bool       `gorm:"column:two_factor_enabled;type:tinyint(1);not null;default:0;index"`
	TwoFactorType      *string    `gorm:"column:two_factor_type;type:varchar(50)"`
	BiometricEnabled   bool       `gorm:"column:biometric_enabled;type:tinyint(1);not null;default:0;index"`
	BiometricType      *string    `gorm:"column:biometric_type;type:varchar(50)"`
	HasTradePassword   bool       `gorm:"column:has_trade_password;type:tinyint(1);not null;default:0;index"`
	TradePasswordHash  string     `gorm:"column:trade_password_hash;type:varchar(255);not null;default:''"`
	LastSecurityUpdate *time.Time `gorm:"column:last_security_update;type:timestamp"`

	LastActiveAt *time.Time `gorm:"column:last_active_at;type:timestamp;index"`

	CreatedAt *time.Time `gorm:"column:created_at;type:timestamp;index"`
	UpdatedAt *time.Time `gorm:"column:updated_at;type:timestamp"`
	DeletedAt *time.Time `gorm:"column:deleted_at;type:timestamp"`
}

func (Web3UserModel) TableName() string { return "web3_users" }
