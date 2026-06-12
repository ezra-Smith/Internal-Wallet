package model

import "time"

// VaultAddressBalanceModel maps to vault_address_balances (balance details per address & currency).
// 存储外部导入的钱包地址余额明细
type VaultAddressBalanceModel struct {
	ID        int64 `gorm:"column:id;primaryKey;autoIncrement"`
	AddressID int64 `gorm:"column:address_id;type:bigint;not null;index"`
	NetworkID int64 `gorm:"column:network_id;type:bigint;not null;index"`

	Currency        string `gorm:"column:currency;type:varchar(20);not null;index"`
	ContractAddress string `gorm:"column:contract_address;type:varchar(255);not null;default:''"`

	Balance    string `gorm:"column:balance;type:varchar(100);not null;default:'0'"`
	BalanceRaw int64  `gorm:"column:balance_raw;type:bigint;not null;default:0;index"`

	BalanceUSD    string `gorm:"column:balance_usd;type:varchar(100);not null;default:'0'"`
	BalanceUSDRaw int64  `gorm:"column:balance_usd_raw;type:bigint;not null;default:0"`

	LastSyncedAt *time.Time `gorm:"column:last_synced_at;type:timestamp"`

	CreatedAt *time.Time `gorm:"column:created_at;type:timestamp"`
	UpdatedAt *time.Time `gorm:"column:updated_at;type:timestamp"`
	DeletedAt *time.Time `gorm:"column:deleted_at;type:timestamp"`
}

func (VaultAddressBalanceModel) TableName() string { return "vault_address_balances" }
