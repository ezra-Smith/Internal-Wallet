package model

import "time"

// WalletDepositModel maps to wallet_deposits (deposit records).
// 存储用户充值记录
type WalletDepositModel struct {
	ID        int64  `gorm:"column:id;primaryKey;autoIncrement"`
	UserID    int64  `gorm:"column:user_id;type:bigint;not null;index"`
	AssetCode string `gorm:"column:asset_code;type:varchar(32);not null;index"`
	ChainCode string `gorm:"column:chain_code;type:varchar(32);not null;index"`

	DepositAddress  string  `gorm:"column:deposit_address;type:varchar(255);not null;index"`
	Amount          string  `gorm:"column:amount;type:decimal(65,30);not null"`
	TransactionHash string  `gorm:"column:transaction_hash;type:varchar(255);not null;index"`
	BlockNumber     *int64  `gorm:"column:block_number;type:bigint;index"`
	Confirmations   int32   `gorm:"column:confirmations;type:int;not null;default:0"`
	Status          string  `gorm:"column:status;type:varchar(20);not null;default:'pending';index"`
	Memo            *string `gorm:"column:memo;type:varchar(255)"`

	CreatedAt *time.Time `gorm:"column:created_at;type:timestamp;index"`
	UpdatedAt *time.Time `gorm:"column:updated_at;type:timestamp"`
	DeletedAt *time.Time `gorm:"column:deleted_at;type:timestamp"`
}

func (WalletDepositModel) TableName() string { return "wallet_deposits" }
