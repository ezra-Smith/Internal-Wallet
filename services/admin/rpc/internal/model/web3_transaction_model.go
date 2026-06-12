package model

import "time"

// Web3TransactionModel maps to web3_transactions (blockchain transaction records).
type Web3TransactionModel struct {
	ID            int64      `gorm:"column:id;primaryKey;autoIncrement"`
	Web3UserID    int64      `gorm:"column:web3_user_id;not null;index"`
	UserAddress   string     `gorm:"column:user_address;type:varchar(255);not null;index"`
	Network       string     `gorm:"column:network;type:varchar(50);not null;index"`
	ChainID       int64      `gorm:"column:chain_id;type:bigint;not null;default:0;index"`
	TxHash        string     `gorm:"column:tx_hash;type:varchar(255);not null;uniqueIndex"`
	BlockNumber   *int64     `gorm:"column:block_number;type:bigint"`
	BlockTime     *time.Time `gorm:"column:block_time;type:timestamp;index"`
	TxType        string     `gorm:"column:tx_type;type:varchar(50);not null;index"`
	Direction     string     `gorm:"column:direction;type:varchar(20);not null;index"`
	AssetCode     string     `gorm:"column:asset_code;type:varchar(50);not null;index"`
	Amount        string     `gorm:"column:amount;type:varchar(100);not null"`
	AmountUSD     *string    `gorm:"column:amount_usd;type:varchar(100)"`
	FromAddress   *string    `gorm:"column:from_address;type:varchar(255)"`
	ToAddress     *string    `gorm:"column:to_address;type:varchar(255)"`
	Status        string     `gorm:"column:status;type:varchar(50);not null;default:'pending';index"`
	Confirmations int32      `gorm:"column:confirmations;type:int;not null;default:0"`
	Fee           *string    `gorm:"column:fee;type:varchar(100)"`
	FeeAsset      *string    `gorm:"column:fee_asset;type:varchar(50)"`
	RawData       []byte     `gorm:"column:raw_data;type:json"`
	CreatedAt     *time.Time `gorm:"column:created_at;type:timestamp;index"`
	UpdatedAt     *time.Time `gorm:"column:updated_at;type:timestamp"`
}

func (Web3TransactionModel) TableName() string {
	return "web3_transactions"
}
