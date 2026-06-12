package model

import "time"

// BlacklistAddressModel maps to blacklist_addresses (on-chain sensitive address library).
// NOTE: This table schema is managed by SQL migrations (do not rely on AutoMigrate).
type BlacklistAddressModel struct {
	ID int64 `gorm:"column:id;primaryKey;autoIncrement"`

	Address string `gorm:"column:address;type:varchar(255);not null;index"`
	Network string `gorm:"column:network;type:varchar(50);not null;index"`

	RiskLevel string  `gorm:"column:risk_level;type:varchar(20);not null;default:'low';index"`
	Source    string  `gorm:"column:source;type:varchar(32);not null;default:'user_report';index"`
	Reason    *string `gorm:"column:reason;type:text"`

	MonitorStatus string     `gorm:"column:monitor_status;type:varchar(16);not null;default:'active';index"`
	HitCount      int64      `gorm:"column:hit_count;type:bigint;not null;default:0"`
	LastHitAt     *time.Time `gorm:"column:last_hit_at;type:timestamp"`

	CreatedByAdminID int64  `gorm:"column:created_by_admin_id;type:bigint;not null;default:0;index"`
	CreatedBy        string `gorm:"column:created_by;type:varchar(128);not null;default:''"`

	CreatedAt *time.Time `gorm:"column:created_at;type:timestamp;index"`
	UpdatedAt *time.Time `gorm:"column:updated_at;type:timestamp"`
	DeletedAt *time.Time `gorm:"column:deleted_at;type:timestamp;index"`
}

func (BlacklistAddressModel) TableName() string { return "blacklist_addresses" }
