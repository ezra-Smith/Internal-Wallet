package model

import (
	commonModel "internalwallet/common/model"
)

// CurrencyGlobalTransferAuditRuleModel maps to `currency_global_transfer_audit_rules`.
// Rules are grouped by asset_code (similar to how withdrawal audit rules are grouped by chain_code).
type CurrencyGlobalTransferAuditRuleModel struct {
	commonModel.BaseModel

	AssetCode string  `gorm:"column:asset_code;type:varchar(32);not null;index" json:"asset_code"`
	MinAmount string  `gorm:"column:min_amount;type:decimal(40,6);not null" json:"min_amount"`     // 统一标准：6位小数（整数34位）
	MaxAmount *string `gorm:"column:max_amount;type:decimal(40,6);default:null" json:"max_amount"` // 统一标准：6位小数（整数34位）
	Strategy  string  `gorm:"column:strategy;type:varchar(16);not null" json:"strategy"`           // auto|manual_auto
	Enabled   bool    `gorm:"column:enabled;type:tinyint;not null;default:1" json:"enabled"`
	SortOrder int32   `gorm:"column:sort_order;type:int;not null;default:0" json:"sort_order"`
	UpdatedBy int64   `gorm:"column:updated_by;type:bigint;not null;default:0" json:"updated_by"`
}

func (CurrencyGlobalTransferAuditRuleModel) TableName() string {
	return "currency_global_transfer_audit_rules"
}
