package model

import (
	commonModel "internalwallet/common/model"
)

// CurrencyGlobalWithdrawAuditRuleModel maps to `currency_global_withdraw_audit_rules`.
type CurrencyGlobalWithdrawAuditRuleModel struct {
	commonModel.BaseModel

	ChainCode string  `gorm:"column:chain_code;type:varchar(32);not null;index" json:"chain_code"`
	MinAmount string  `gorm:"column:min_amount;type:decimal(40,6);not null" json:"min_amount"`     // 统一标准：6位小数（整数34位）
	MaxAmount *string `gorm:"column:max_amount;type:decimal(40,6);default:null" json:"max_amount"` // 统一标准：6位小数（整数34位）
	Strategy  string  `gorm:"column:strategy;type:varchar(16);not null" json:"strategy"`           // auto|manual_auto|manual_manual
	Enabled   bool    `gorm:"column:enabled;type:tinyint;not null;default:1" json:"enabled"`
	SortOrder int32   `gorm:"column:sort_order;type:int;not null;default:0" json:"sort_order"`
	UpdatedBy int64   `gorm:"column:updated_by;type:bigint;not null;default:0" json:"updated_by"`
}

func (CurrencyGlobalWithdrawAuditRuleModel) TableName() string {
	return "currency_global_withdraw_audit_rules"
}
