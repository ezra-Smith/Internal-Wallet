package model

import (
	"time"

	"gorm.io/gorm"
)

// CurrencyWithdrawPayoutTaskModel maps to `currency_withdraw_payout_tasks`.
type CurrencyWithdrawPayoutTaskModel struct {
	WithdrawOrderID int64   `gorm:"column:withdraw_order_id;primaryKey;autoIncrement:false"`
	Mode            string  `gorm:"column:mode;type:varchar(16);not null;default:'system'"`
	State           string  `gorm:"column:state;type:varchar(32);not null;default:'pending_broadcast';index"`
	TxHash          *string `gorm:"column:tx_hash;type:varchar(128);default:null;index"`

	BroadcastAttempts    int `gorm:"column:broadcast_attempts;type:int;not null;default:0"`
	MaxBroadcastAttempts int `gorm:"column:max_broadcast_attempts;type:int;not null;default:3"`
	ConfirmChecks        int `gorm:"column:confirm_checks;type:int;not null;default:0"`
	SettleAttempts       int `gorm:"column:settle_attempts;type:int;not null;default:0"`

	NextRetryTime time.Time  `gorm:"column:next_retry_time;type:datetime;not null;index"`
	LockOwner     *string    `gorm:"column:lock_owner;type:varchar(64);default:null"`
	LockUntil     *time.Time `gorm:"column:lock_until;type:datetime;default:null;index"`
	LastError     *string    `gorm:"column:last_error;type:varchar(512);default:null"`

	CreatedAt time.Time      `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time      `gorm:"column:updated_at;autoUpdateTime"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (CurrencyWithdrawPayoutTaskModel) TableName() string { return "currency_withdraw_payout_tasks" }
