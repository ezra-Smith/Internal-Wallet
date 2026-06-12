package model

import (
	commonModel "internalwallet/common/model"
)

// SwapConfigModel 简化的Swap配置模型（扁平化：provider+chain+token+params）
// 对应数据库表：swap_configs
type SwapConfigModel struct {
	commonModel.BaseModel

	// 服务商关联
	ProviderID   int64  `gorm:"column:provider_id;type:bigint;not null;index:idx_provider_id" json:"provider_id"`
	Provider     string `gorm:"column:provider;type:varchar(32);not null;index:idx_provider" json:"provider"` // 冗余字段，优化查询
	ProviderName string `gorm:"column:provider_name;type:varchar(64);not null" json:"provider_name"`          // 冗余字段，优化查询

	// 链信息
	ChainID       int64  `gorm:"column:chain_id;type:bigint;not null;index:idx_chain" json:"chain_id"`
	ChainName     string `gorm:"column:chain_name;type:varchar(64);not null" json:"chain_name"`
	ChainSymbol   string `gorm:"column:chain_symbol;type:varchar(16);not null" json:"chain_symbol"`
	RouterAddress string `gorm:"column:router_address;type:varchar(128);not null;default:''" json:"router_address"`

	// 代币信息
	TokenSymbol     string `gorm:"column:token_symbol;type:varchar(32);not null;index:idx_token" json:"token_symbol"`
	TokenName       string `gorm:"column:token_name;type:varchar(64);not null" json:"token_name"`
	ContractAddress string `gorm:"column:contract_address;type:varchar(128);not null;default:''" json:"contract_address"`
	Decimals        int    `gorm:"column:decimals;type:int;not null;default:18" json:"decimals"`
	IconURL         string `gorm:"column:icon_url;type:varchar(255)" json:"icon_url"`

	// 全局参数
	IsEnabled        bool   `gorm:"column:is_enabled;type:tinyint(1);not null;default:0" json:"is_enabled"`
	Priority         int    `gorm:"column:priority;type:int;not null;default:0" json:"priority"`
	MinSwapAmountUsd string `gorm:"column:min_swap_amount_usd;type:decimal(36,18);not null;default:10" json:"min_swap_amount_usd"`
	MaxSwapAmountUsd string `gorm:"column:max_swap_amount_usd;type:decimal(36,18);not null;default:100000" json:"max_swap_amount_usd"`
	DefaultSlippage  string `gorm:"column:default_slippage;type:decimal(10,6);not null;default:0.5" json:"default_slippage"`
	MaxSlippage      string `gorm:"column:max_slippage;type:decimal(10,6);not null;default:5.0" json:"max_slippage"`
	FeeRate          string `gorm:"column:fee_rate;type:decimal(10,6);not null;default:0.25" json:"fee_rate"`

	// 额外配置
	ConfigJSON string `gorm:"column:config_json;type:json" json:"config_json"`
}

// TableName 指定表名
func (SwapConfigModel) TableName() string {
	return "swap_configs"
}

// IsNativeToken 判断是否为原生代币（无合约地址）
func (m *SwapConfigModel) IsNativeToken() bool {
	return m.ContractAddress == "" || m.ContractAddress == "0x0000000000000000000000000000000000000000"
}

// GetUniqueKey 获取唯一标识（用于去重）
func (m *SwapConfigModel) GetUniqueKey() string {
	return m.Provider + "_" + string(rune(m.ChainID)) + "_" + m.ContractAddress
}
