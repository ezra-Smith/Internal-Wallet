package model

import (
	"time"

	"gorm.io/gorm"
)

// AcctAccountModel maps to `acct_accounts`.
type AcctAccountModel struct {
	ID              int64  `gorm:"column:id;primaryKey"`
	OwnerType       string `gorm:"column:owner_type;type:varchar(16);not null;index"`
	OwnerID         int64  `gorm:"column:owner_id;type:bigint;not null;default:0;index"`
	AccountTypeCode string `gorm:"column:account_type_code;type:varchar(64);not null;index"`
	ChainScope      string `gorm:"column:chain_scope;type:varchar(32);not null;default:'';index"`

	CreatedAt time.Time      `gorm:"column:created_at;type:datetime;autoCreateTime"`
	UpdatedAt time.Time      `gorm:"column:updated_at;type:datetime;autoUpdateTime"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (AcctAccountModel) TableName() string { return "acct_accounts" }
