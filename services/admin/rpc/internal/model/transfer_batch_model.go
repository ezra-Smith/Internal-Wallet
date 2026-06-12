package model

import "time"

// TransferBatchModel maps to transfer_batches.
type TransferBatchModel struct {
	BatchID         string     `gorm:"column:batch_id;type:varchar(32);primaryKey"`
	Name            string     `gorm:"column:name;type:varchar(128);not null"`
	TransferType    string     `gorm:"column:transfer_type;type:varchar(16);not null;default:'normal';index"`
	Description     *string    `gorm:"column:description;type:varchar(512)"`
	Network         string     `gorm:"column:network;type:varchar(32);not null;index"`
	Currency        string     `gorm:"column:currency;type:varchar(16);not null;index"`
	TotalRecipients int32      `gorm:"column:total_recipients;type:int;not null;default:0"`
	TotalAmount     string     `gorm:"column:total_amount;type:decimal(36,18);not null;default:'0'"`
	TotalAmountUSD  string     `gorm:"column:total_amount_usd;type:decimal(36,18);not null;default:'0'"`
	SuccessCount    int32      `gorm:"column:success_count;type:int;not null;default:0"`
	FailedCount     int32      `gorm:"column:failed_count;type:int;not null;default:0"`
	Status          string     `gorm:"column:status;type:varchar(16);not null;default:'draft';index"`
	CreatedBy       string     `gorm:"column:created_by;type:varchar(128);not null;default:''"`
	CreatedByID     int64      `gorm:"column:created_by_id;type:bigint;not null;default:0;index"`
	CreatedAt       *time.Time `gorm:"column:created_at;type:datetime;not null;index"`
	SubmittedAt     *time.Time `gorm:"column:submitted_at;type:datetime"`
	ApprovedBy      *string    `gorm:"column:approved_by;type:varchar(128)"`
	ApprovedByID    int64      `gorm:"column:approved_by_id;type:bigint;not null;default:0"`
	ApprovedAt      *time.Time `gorm:"column:approved_at;type:datetime"`
	ProcessedAt     *time.Time `gorm:"column:processed_at;type:datetime"`
	CompletedAt     *time.Time `gorm:"column:completed_at;type:datetime"`
	CancelledAt     *time.Time `gorm:"column:cancelled_at;type:datetime"`
	CancelledBy     *string    `gorm:"column:cancelled_by;type:varchar(128)"`
	CancelledByID   int64      `gorm:"column:cancelled_by_id;type:bigint;not null;default:0"`
	CancelReason    *string    `gorm:"column:cancel_reason;type:varchar(512)"`
}

func (TransferBatchModel) TableName() string { return "transfer_batches" }
