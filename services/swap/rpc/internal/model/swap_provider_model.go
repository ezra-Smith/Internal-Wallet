package model

import (
	commonModel "internalwallet/common/model"
)

// SwapProviderModel Swap服务商配置模型
// 对应数据库表：swap_provider
type SwapProviderModel struct {
	commonModel.BaseModel

	// 服务商标识
	ProviderCode string `gorm:"column:provider_code;type:varchar(32);not null;index:idx_provider_code" json:"provider_code"`
	ProviderName string `gorm:"column:provider_name;type:varchar(64);not null" json:"provider_name"`
	Description  string `gorm:"column:description;type:varchar(255)" json:"description"`
	LogoURL      string `gorm:"column:logo_url;type:varchar(255)" json:"logo_url"`

	// 服务商总开关
	IsEnabled bool `gorm:"column:is_enabled;type:tinyint(1);not null;default:1" json:"is_enabled"`

	// 全局默认参数（所有币对继承）
	MinSwapAmountUsd string `gorm:"column:min_swap_amount_usd;type:decimal(36,18);not null;default:10" json:"min_swap_amount_usd"`
	MaxSwapAmountUsd string `gorm:"column:max_swap_amount_usd;type:decimal(36,18);not null;default:100000" json:"max_swap_amount_usd"`
	DefaultSlippage  string `gorm:"column:default_slippage;type:decimal(10,6);not null;default:0.5" json:"default_slippage"`
	MaxSlippage      string `gorm:"column:max_slippage;type:decimal(10,6);not null;default:5.0" json:"max_slippage"`
	FeeRate          string `gorm:"column:fee_rate;type:decimal(10,6);not null;default:0.25" json:"fee_rate"`

	// 额外配置
	ConfigJSON string `gorm:"column:config_json;type:json" json:"config_json"`
}

// TableName 指定表名
func (SwapProviderModel) TableName() string {
	return "swap_provider"
}
