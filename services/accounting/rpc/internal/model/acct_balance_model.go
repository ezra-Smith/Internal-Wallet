package model

import (
	"time"

	"gorm.io/gorm"
)

// AcctBalanceModel maps to `acct_balances`.
type AcctBalanceModel struct {
	ID        int64  `gorm:"column:id;primaryKey;autoIncrement"`
	AccountID int64  `gorm:"column:account_id;type:bigint;not null;index;uniqueIndex:uk_acct_balances_account_asset_bucket,priority:1"`
	AssetCode string `gorm:"column:asset_code;type:varchar(32);not null;index;uniqueIndex:uk_acct_balances_account_asset_bucket,priority:2"`
	Bucket    string `gorm:"column:bucket;type:varchar(16);not null;index;uniqueIndex:uk_acct_balances_account_asset_bucket,priority:3"`

	// balance_raw is DECIMAL(65,0) in DB; keep as string to avoid int64 overflow.
	BalanceRaw string `gorm:"column:balance_raw;type:decimal(65,0);not null;default:0"`

	CreatedAt time.Time      `gorm:"column:created_at;type:datetime;autoCreateTime"`
	UpdatedAt time.Time      `gorm:"column:updated_at;type:datetime;autoUpdateTime"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (AcctBalanceModel) TableName() string { return "acct_balances" }
