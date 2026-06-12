package model

import (
	"time"

	"internalwallet/common/utils"

	"gorm.io/gorm"
)

// CurrencyWithdrawOrderEventModel maps to `currency_withdraw_order_events`.
// This table is append-only for audit purposes.
type CurrencyWithdrawOrderEventModel struct {
	ID              int64   `gorm:"column:id;primaryKey;autoIncrement:false"`
	WithdrawOrderID int64   `gorm:"column:withdraw_order_id;type:bigint;not null;index"`
	EventType       string  `gorm:"column:event_type;type:varchar(64);not null;index"`
	ActorType       string  `gorm:"column:actor_type;type:varchar(16);not null;index"`
	ActorID         int64   `gorm:"column:actor_id;type:bigint;not null;default:0"`
	IP              *string `gorm:"column:ip;type:varchar(64);default:null"`
	Summary         string  `gorm:"column:summary;type:varchar(255);not null"`
	Details         []byte  `gorm:"column:details;type:json"`

	CreatedAt time.Time      `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time      `gorm:"column:updated_at;autoUpdateTime"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (CurrencyWithdrawOrderEventModel) TableName() string { return "currency_withdraw_order_events" }

func (m *CurrencyWithdrawOrderEventModel) BeforeCreate(tx *gorm.DB) error {
	if m.ID == 0 {
		m.ID = utils.GenerateID()
	}
	now := time.Now().Local()
	if m.CreatedAt.IsZero() {
		m.CreatedAt = now
	}
	if m.UpdatedAt.IsZero() {
		m.UpdatedAt = m.CreatedAt
	}
	m.DeletedAt = gorm.DeletedAt{}
	return nil
}
