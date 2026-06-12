package model

import "time"

// WalletDepositAddressModel maps to wallet_user_chain_addresses (admin-managed user deposit addresses; chain-level).
// 用户充值地址 signer 生成（同一用户同一条链最多一个 active 地址）
type WalletDepositAddressModel struct {
	ID        int64  `gorm:"column:id;primaryKey;autoIncrement"`
	UserID    int64  `gorm:"column:user_id;type:bigint;not null;index"`
	ChainCode string `gorm:"column:chain_code;type:varchar(32);not null;index"`

	Address string  `gorm:"column:address;type:varchar(255);not null;index"`
	Status  string  `gorm:"column:status;type:varchar(20);not null;default:'active';index"`
	Memo    *string `gorm:"column:memo;type:varchar(255)"`

	Label            *string    `gorm:"column:label;type:varchar(100)"`
	Remark           *string    `gorm:"column:remark;type:varchar(500)"`
	IsDefault        bool       `gorm:"column:is_default;type:tinyint(1);not null;default:0;index"`
	DerivationChange int        `gorm:"column:derivation_change;type:int;not null;default:0;index"`
	LastUsedAt       *time.Time `gorm:"column:last_used_at;type:timestamp"`

	CreatedAt *time.Time `gorm:"column:created_at;type:timestamp"`
	UpdatedAt *time.Time `gorm:"column:updated_at;type:timestamp"`
	DeletedAt *time.Time `gorm:"column:deleted_at;type:timestamp"`
}

func (WalletDepositAddressModel) TableName() string { return "wallet_user_chain_addresses" }
