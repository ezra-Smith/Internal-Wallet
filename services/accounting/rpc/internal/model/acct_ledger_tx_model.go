package model

import (
	"time"

	"gorm.io/gorm"
)

// AcctLedgerTxModel maps to `acct_ledger_tx`.
type AcctLedgerTxModel struct {
	ID             int64  `gorm:"column:id;primaryKey"`
	OpType         string `gorm:"column:op_type;type:varchar(64);not null;index"`
	BizRef         string `gorm:"column:biz_ref;type:varchar(128);not null;default:'';index"`
	IdempotencyKey string `gorm:"column:idempotency_key;type:varchar(128);not null;uniqueIndex"`
	RequestHash    string `gorm:"column:request_hash;type:char(64);not null"`

	CreatedAt time.Time      `gorm:"column:created_at;type:datetime;autoCreateTime"`
	UpdatedAt time.Time      `gorm:"column:updated_at;type:datetime;autoUpdateTime"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (AcctLedgerTxModel) TableName() string { return "acct_ledger_tx" }
