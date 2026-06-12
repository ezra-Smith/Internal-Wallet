package model

import "time"

// VaultBalanceModel maps to vault_balances (current vault balance per network & currency/contract).
// 网络总余额,聚合自 vault_address_balances
type VaultBalanceModel struct {
	ID        int64 `gorm:"column:id;primaryKey;autoIncrement"`
	NetworkID int64 `gorm:"column:network_id;type:bigint;not null;index"`

	Currency        string `gorm:"column:currency;type:varchar(20);not null;index"`
	ContractAddress string `gorm:"column:contract_address;type:varchar(255);not null;default:'';index"`

	Balance    string `gorm:"column:balance;type:varchar(100);not null"`
	BalanceRaw int64  `gorm:"column:balance_raw;type:bigint;not null;index"`

	BalanceUSD    string `gorm:"column:balance_usd;type:varchar(100);not null"`
	BalanceUSDRaw int64  `gorm:"column:balance_usd_raw;type:bigint;not null;index"`

	LastUpdatedAt *time.Time `gorm:"column:last_updated_at;type:timestamp"`

	CreatedAt *time.Time `gorm:"column:created_at;type:timestamp"`
	UpdatedAt *time.Time `gorm:"column:updated_at;type:timestamp"`
	DeletedAt *time.Time `gorm:"column:deleted_at;type:timestamp"`
}

func (VaultBalanceModel) TableName() string { return "vault_balances" }
