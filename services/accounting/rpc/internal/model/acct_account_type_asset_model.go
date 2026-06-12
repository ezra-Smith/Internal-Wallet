package model

import (
	"time"

	"gorm.io/gorm"
)

// AcctAccountTypeAssetModel maps to `acct_account_type_assets`.
type AcctAccountTypeAssetModel struct {
	ID              int64          `gorm:"column:id;primaryKey;autoIncrement"`
	AccountTypeCode string         `gorm:"column:account_type_code;type:varchar(64);not null;index;uniqueIndex:uk_acct_account_type_assets_type_asset,priority:1"`
	AssetCode       string         `gorm:"column:asset_code;type:varchar(32);not null;index;uniqueIndex:uk_acct_account_type_assets_type_asset,priority:2"`
	CreatedAt       time.Time      `gorm:"column:created_at;type:datetime;autoCreateTime"`
	UpdatedAt       time.Time      `gorm:"column:updated_at;type:datetime;autoUpdateTime"`
	DeletedAt       gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (AcctAccountTypeAssetModel) TableName() string { return "acct_account_type_assets" }
