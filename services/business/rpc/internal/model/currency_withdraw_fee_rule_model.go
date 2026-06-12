package model

import (
	commonModel "internalwallet/common/model"
)

// CurrencyWithdrawFeeRuleModel maps to `currency_withdraw_fee_rules`.
type CurrencyWithdrawFeeRuleModel struct {
	commonModel.BaseModel

	AssetCode string  `gorm:"column:asset_code;type:varchar(32);not null;index"`
	ChainCode string  `gorm:"column:chain_code;type:varchar(32);not null;index"`
	RuleType  string  `gorm:"column:rule_type;type:varchar(16);not null"`
	Value     string  `gorm:"column:value;type:decimal(40,6);not null"`
	MinFee    *string `gorm:"column:min_fee;type:decimal(40,6);default:null"`
	MaxFee    *string `gorm:"column:max_fee;type:decimal(40,6);default:null"`
	MinAmount *string `gorm:"column:min_amount;type:decimal(40,6);default:null"`
	MaxAmount *string `gorm:"column:max_amount;type:decimal(40,6);default:null"`
	Enabled   bool    `gorm:"column:enabled;type:tinyint;not null;default:1"`
	SortOrder int32   `gorm:"column:sort_order;type:int;not null;default:0"`
	UpdatedBy int64   `gorm:"column:updated_by;type:bigint;not null;default:0"`
}

func (CurrencyWithdrawFeeRuleModel) TableName() string { return "currency_withdraw_fee_rules" }
