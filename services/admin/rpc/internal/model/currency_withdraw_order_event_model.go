package model

import (
	"time"

	"internalwallet/common/utils"

	"gorm.io/gorm"
)

// CurrencyWithdrawOrderEventModel maps to `currency_withdraw_order_events`.
// This table is append-only for audit purposes.
type CurrencyWithdrawOrderEventModel struct {
	ID              int64   `gorm:"column:id;primaryKey;autoIncrement:false" json:"id"`
	WithdrawOrderID int64   `gorm:"column:withdraw_order_id;type:bigint;not null;index" json:"withdraw_order_id"`
	EventType       string  `gorm:"column:event_type;type:varchar(64);not null;index" json:"event_type"`
	ActorType       string  `gorm:"column:actor_type;type:varchar(16);not null;index" json:"actor_type"`
	ActorID         int64   `gorm:"column:actor_id;type:bigint;not null;default:0" json:"actor_id"`
	IP              *string `gorm:"column:ip;type:varchar(64);default:null" json:"ip"`
	Summary         string  `gorm:"column:summary;type:varchar(255);not null" json:"summary"`
	Details         []byte  `gorm:"column:details;type:json" json:"details"`

	CreatedAt time.Time      `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index" json:"deleted_at"`
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
