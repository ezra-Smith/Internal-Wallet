package model

import (
	commonModel "internalwallet/common/model"
)

// CurrencyWithdrawAuditRuleModel maps to `currency_withdraw_audit_rules`.
type CurrencyWithdrawAuditRuleModel struct {
	commonModel.BaseModel

	AssetCode string `gorm:"column:asset_code;type:varchar(32);not null;index"`
	ChainCode string `gorm:"column:chain_code;type:varchar(32);not null;index"`

	MinAmount string  `gorm:"column:min_amount;type:decimal(40,6);not null"`
	MaxAmount *string `gorm:"column:max_amount;type:decimal(40,6);default:null"`
	Strategy  string  `gorm:"column:strategy;type:varchar(16);not null"` // auto|manual_auto|manual_manual
	Enabled   bool    `gorm:"column:enabled;type:tinyint;not null;default:1"`
	SortOrder int32   `gorm:"column:sort_order;type:int;not null;default:0"`
	UpdatedBy int64   `gorm:"column:updated_by;type:bigint;not null;default:0"`
}

func (CurrencyWithdrawAuditRuleModel) TableName() string { return "currency_withdraw_audit_rules" }
