package model

import "time"

// TransferBatchItemModel maps to transfer_batch_items.
type TransferBatchItemModel struct {
	ID           string     `gorm:"column:id;type:varchar(32);primaryKey"`
	BatchID      string     `gorm:"column:batch_id;type:varchar(32);not null;index"`
	UserID       *int64     `gorm:"column:user_id;type:bigint;index"`
	Currency     string     `gorm:"column:currency;type:varchar(16);not null;default:'';index"`
	ToAddress    string     `gorm:"column:to_address;type:varchar(128);not null;index"`
	Amount       string     `gorm:"column:amount;type:decimal(36,18);not null"`
	Note         *string    `gorm:"column:note;type:varchar(256)"`
	Status       string     `gorm:"column:status;type:varchar(16);not null;default:'pending';index"`
	TxHash       *string    `gorm:"column:tx_hash;type:varchar(128)"`
	Fee          *string    `gorm:"column:fee;type:decimal(36,18)"`
	LedgerTxID   *int64     `gorm:"column:ledger_tx_id;type:bigint"`
	ErrorMessage *string    `gorm:"column:error_message;type:varchar(512)"`
	RetryCount   int32      `gorm:"column:retry_count;type:int;not null;default:0"`
	CreatedAt    *time.Time `gorm:"column:created_at;type:datetime;not null"`
	ProcessedAt  *time.Time `gorm:"column:processed_at;type:datetime"`
}

func (TransferBatchItemModel) TableName() string { return "transfer_batch_items" }
