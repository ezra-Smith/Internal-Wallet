package model

import "time"

// VaultThresholdModel maps to vault_thresholds (alert threshold per network & currency).
type VaultThresholdModel struct {
	ID        int64 `gorm:"column:id;primaryKey;autoIncrement"`
	NetworkID int64 `gorm:"column:network_id;type:bigint;not null;index"`

	Currency string `gorm:"column:currency;type:varchar(20);not null;index"`

	ThresholdLow         string `gorm:"column:threshold_low;type:varchar(100);not null"`
	ThresholdLowRaw      int64  `gorm:"column:threshold_low_raw;type:bigint;not null"`
	ThresholdCritical    string `gorm:"column:threshold_critical;type:varchar(100);not null"`
	ThresholdCriticalRaw int64  `gorm:"column:threshold_critical_raw;type:bigint;not null"`

	// Notifications JSON (serialized bytes).
	Notifications []byte `gorm:"column:notifications;type:json"`

	UpdatedBy int64 `gorm:"column:updated_by;type:bigint;not null;default:0;index"`

	CreatedAt *time.Time `gorm:"column:created_at;type:timestamp"`
	UpdatedAt *time.Time `gorm:"column:updated_at;type:timestamp"`
	DeletedAt *time.Time `gorm:"column:deleted_at;type:timestamp"`
}

func (VaultThresholdModel) TableName() string { return "vault_thresholds" }
