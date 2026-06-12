package model

import (
	commonModel "internalwallet/common/model"
)

// CurrencyChainSettingsModel maps to `currency_chain_settings`.
type CurrencyChainSettingsModel struct {
	commonModel.BaseModel

	AssetCode       string  `gorm:"column:asset_code;type:varchar(32);not null;index"`
	ChainCode       string  `gorm:"column:chain_code;type:varchar(32);not null;index"`
	ContractAddress *string `gorm:"column:contract_address;type:varchar(255);default:null"`
	// TokenDecimals is the on-chain token decimals (e.g., 18 for BSC USDT, 6 for ETH USDT)
	// This is different from Accounting service's asset precision (always 6 for USDT)
	TokenDecimals           *int32 `gorm:"column:token_decimals;type:int;default:null"`
	Web3AssetDisplayEnabled bool   `gorm:"column:web3_asset_display_enabled;type:tinyint;not null"`

	DepositEnabled  bool  `gorm:"column:deposit_enabled;type:tinyint;not null;default:1"`
	WithdrawEnabled bool  `gorm:"column:withdraw_enabled;type:tinyint;not null;default:1"`
	Status          int32 `gorm:"column:status;type:tinyint;not null;default:1;index"` // 1=enabled 2=disabled

	MinWithdrawAmount *string `gorm:"column:min_withdraw_amount;type:decimal(40,6);default:null"`
	MinDepositAmount  *string `gorm:"column:min_deposit_amount;type:decimal(40,6);default:null"`

	UpdatedBy int64 `gorm:"column:updated_by;type:bigint;not null;default:0"`
}

func (CurrencyChainSettingsModel) TableName() string { return "currency_chain_settings" }
