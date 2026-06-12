package model

import (
	commonModel "internalwallet/common/model"
)

// CurrencySettingsModel maps to `currency_settings`.
type CurrencySettingsModel struct {
	commonModel.BaseModel

	AssetCode string `gorm:"column:asset_code;type:varchar(32);not null;index"`

	Web2DepositEnabled  bool `gorm:"column:web2_deposit_enabled;type:tinyint;not null;default:0"`
	Web2WithdrawEnabled bool `gorm:"column:web2_withdraw_enabled;type:tinyint;not null;default:0"`
	Web2TransferEnabled bool `gorm:"column:web2_transfer_enabled;type:tinyint;not null;default:0"`

	Web3DepositEnabled  bool `gorm:"column:web3_deposit_enabled;type:tinyint;not null;default:1"`
	Web3WithdrawEnabled bool `gorm:"column:web3_withdraw_enabled;type:tinyint;not null;default:1"`
	Web3SwapEnabled     bool `gorm:"column:web3_swap_enabled;type:tinyint;not null;default:0"`

	UseGlobalWithdrawFee   bool  `gorm:"column:use_global_withdraw_fee;type:tinyint;not null;default:1"`
	UseGlobalWithdrawAudit bool  `gorm:"column:use_global_withdraw_audit;type:tinyint;not null;default:1"`
	UseGlobalTransferAudit bool  `gorm:"column:use_global_transfer_audit;type:tinyint;not null;default:1"`
	UpdatedBy              int64 `gorm:"column:updated_by;type:bigint;not null;default:0"`
}

func (CurrencySettingsModel) TableName() string { return "currency_settings" }
