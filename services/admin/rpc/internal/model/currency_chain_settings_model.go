package model

import (
	commonModel "internalwallet/common/model"
)

// CurrencyChainSettingsModel maps to `currency_chain_settings`.
type CurrencyChainSettingsModel struct {
	commonModel.BaseModel

	AssetCode       string  `gorm:"column:asset_code;type:varchar(32);not null;index" json:"asset_code"`
	ChainCode       string  `gorm:"column:chain_code;type:varchar(32);not null;index" json:"chain_code"`
	ContractAddress *string `gorm:"column:contract_address;type:varchar(255);default:null" json:"contract_address"`
	// TokenDecimals is required for token assets (contract_address not empty).
	// For native tokens, this should be NULL.
	TokenDecimals *int32 `gorm:"column:token_decimals;type:int;default:null" json:"token_decimals,omitempty"`
	// Web3AssetDisplayEnabled controls whether this asset is displayed under Web3 for this chain.
	// NOTE:
	// - DB column has DEFAULT 1 (true) for backward compatibility.
	// - DO NOT set `default:1` in GORM tag here; otherwise `false` (zero value) may be overwritten by default on Create.
	Web3AssetDisplayEnabled bool `gorm:"column:web3_asset_display_enabled;type:tinyint;not null" json:"web3_asset_display_enabled"`

	// ConsolidationEnabled controls whether consolidation is enabled for this asset+chain.
	// DB column has DEFAULT 0 (false).
	ConsolidationEnabled bool `gorm:"column:consolidation_enabled;type:tinyint;not null" json:"consolidation_enabled"`

	DepositEnabled  bool  `gorm:"column:deposit_enabled;type:tinyint;not null;default:1" json:"deposit_enabled"`
	WithdrawEnabled bool  `gorm:"column:withdraw_enabled;type:tinyint;not null;default:1" json:"withdraw_enabled"`
	Status          int32 `gorm:"column:status;type:tinyint;not null;default:1;index" json:"status"` // 1=enabled 2=disabled

	MinWithdrawAmount *string `gorm:"column:min_withdraw_amount;type:decimal(40,6);default:null" json:"min_withdraw_amount"`
	MinDepositAmount  *string `gorm:"column:min_deposit_amount;type:decimal(40,6);default:null" json:"min_deposit_amount"`

	UpdatedBy int64 `gorm:"column:updated_by;type:bigint;not null;default:0" json:"updated_by"`
}

func (CurrencyChainSettingsModel) TableName() string { return "currency_chain_settings" }
