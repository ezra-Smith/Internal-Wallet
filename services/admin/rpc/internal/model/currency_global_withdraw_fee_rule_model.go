package model

import (
	commonModel "internalwallet/common/model"
)

// CurrencyGlobalWithdrawFeeRuleModel maps to `currency_global_withdraw_fee_rules`.
type CurrencyGlobalWithdrawFeeRuleModel struct {
	commonModel.BaseModel

	ChainCode string  `gorm:"column:chain_code;type:varchar(32);not null;index" json:"chain_code"`
	RuleType  string  `gorm:"column:rule_type;type:varchar(16);not null" json:"rule_type"`         // fixed|percent
	Value     string  `gorm:"column:value;type:decimal(40,6);not null" json:"value"`               // 统一标准：6位小数（整数34位）
	MinFee    *string `gorm:"column:min_fee;type:decimal(40,6);default:null" json:"min_fee"`       // 统一标准：6位小数（整数34位）
	MaxFee    *string `gorm:"column:max_fee;type:decimal(40,6);default:null" json:"max_fee"`       // 统一标准：6位小数（整数34位）
	MinAmount *string `gorm:"column:min_amount;type:decimal(40,6);default:null" json:"min_amount"` // 规则适用的最小提现金额
	MaxAmount *string `gorm:"column:max_amount;type:decimal(40,6);default:null" json:"max_amount"` // 规则适用的最大提现金额
	Enabled   bool    `gorm:"column:enabled;type:tinyint;not null;default:1" json:"enabled"`
	SortOrder int32   `gorm:"column:sort_order;type:int;not null;default:0" json:"sort_order"`
	UpdatedBy int64   `gorm:"column:updated_by;type:bigint;not null;default:0" json:"updated_by"`
}

func (CurrencyGlobalWithdrawFeeRuleModel) TableName() string {
	return "currency_global_withdraw_fee_rules"
}
