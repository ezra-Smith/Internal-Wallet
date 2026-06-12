package model

import (
	commonModel "internalwallet/common/model"
)

// CurrencyWithdrawFeeRuleModel maps to `currency_withdraw_fee_rules`.
type CurrencyWithdrawFeeRuleModel struct {
	commonModel.BaseModel

	AssetCode string  `gorm:"column:asset_code;type:varchar(32);not null;index" json:"asset_code"`
	ChainCode string  `gorm:"column:chain_code;type:varchar(32);not null;index" json:"chain_code"`
	RuleType  string  `gorm:"column:rule_type;type:varchar(16);not null" json:"rule_type"`         // fixed|percent
	Value     string  `gorm:"column:value;type:decimal(40,6);not null" json:"value"`               // 统一标准：6位小数（整数34位）
	MinFee    *string `gorm:"column:min_fee;type:decimal(40,6);default:null" json:"min_fee"`       // 手续费下限
	MaxFee    *string `gorm:"column:max_fee;type:decimal(40,6);default:null" json:"max_fee"`       // 手续费上限
	MinAmount *string `gorm:"column:min_amount;type:decimal(40,6);default:null" json:"min_amount"` // 金额区间下限
	MaxAmount *string `gorm:"column:max_amount;type:decimal(40,6);default:null" json:"max_amount"` // 金额区间上限
	Enabled   bool    `gorm:"column:enabled;type:tinyint;not null;default:1" json:"enabled"`
	SortOrder int32   `gorm:"column:sort_order;type:int;not null;default:0" json:"sort_order"`
	UpdatedBy int64   `gorm:"column:updated_by;type:bigint;not null;default:0" json:"updated_by"`
}

func (CurrencyWithdrawFeeRuleModel) TableName() string { return "currency_withdraw_fee_rules" }
