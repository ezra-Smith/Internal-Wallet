package model

import "time"

// Web3UserAddressModel maps to web3_user_addresses (addresses per device).
type Web3UserAddressModel struct {
	ID         int64 `gorm:"column:id;primaryKey;autoIncrement"`
	Web3UserID int64 `gorm:"column:web3_user_id;type:bigint;not null;index"`

	Network string `gorm:"column:network;type:varchar(50);not null;index"`
	ChainID int64  `gorm:"column:chain_id;type:bigint;not null;default:0;index"`
	Address string `gorm:"column:address;type:varchar(255);not null;index"`

	IsPrimary bool `gorm:"column:is_primary;type:tinyint(1);not null;default:0;index"`
	Enabled   bool `gorm:"column:enabled;type:tinyint(1);not null;default:1;index"`

	IsBlacklisted   bool       `gorm:"column:is_blacklisted;type:tinyint(1);not null;default:0;index"`
	BlacklistReason *string    `gorm:"column:blacklist_reason;type:text"`
	RiskLevel       *string    `gorm:"column:risk_level;type:varchar(20)"`
	BlacklistedAt   *time.Time `gorm:"column:blacklisted_at;type:timestamp"`
	BlacklistedBy   *string    `gorm:"column:blacklisted_by;type:varchar(100)"`
	RemovedAt       *time.Time `gorm:"column:removed_at;type:timestamp"`
	RemovedBy       *string    `gorm:"column:removed_by;type:varchar(100)"`
	RemoveReason    *string    `gorm:"column:remove_reason;type:text"`

	Balance           string     `gorm:"column:balance;type:varchar(100);not null;default:'0'"`
	BalanceUSD        string     `gorm:"column:balance_usd;type:varchar(100);not null;default:'0'"`
	TokenBalances     []byte     `gorm:"column:token_balances;type:json"`
	LastTransactionAt *time.Time `gorm:"column:last_transaction_at;type:timestamp;index"`

	CreatedAt *time.Time `gorm:"column:created_at;type:timestamp;index"`
	UpdatedAt *time.Time `gorm:"column:updated_at;type:timestamp"`
	DeletedAt *time.Time `gorm:"column:deleted_at;type:timestamp"`
}

func (Web3UserAddressModel) TableName() string { return "web3_user_addresses" }
