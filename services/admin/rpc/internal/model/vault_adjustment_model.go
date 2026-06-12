package model

import "time"

// VaultAdjustmentModel maps to vault_adjustments (manual balance adjustments with approvals).
type VaultAdjustmentModel struct {
	ID        int64  `gorm:"column:id;primaryKey;autoIncrement"`
	NetworkID int64  `gorm:"column:network_id;type:bigint;not null;index"`
	ChainID   int64  `gorm:"column:chain_id;type:bigint;not null;index"`
	Network   string `gorm:"column:network;type:varchar(32);not null;index"`

	Currency string `gorm:"column:currency;type:varchar(20);not null;index"`

	AdjustmentType string `gorm:"column:adjustment_type;type:varchar(20);not null;index"`

	Amount    string `gorm:"column:amount;type:varchar(100);not null"`
	AmountRaw int64  `gorm:"column:amount_raw;type:bigint;not null;index"`

	AmountUSD    string `gorm:"column:amount_usd;type:varchar(100);not null"`
	AmountUSDRaw int64  `gorm:"column:amount_usd_raw;type:bigint;not null"`

	Reason string `gorm:"column:reason;type:varchar(255);not null"`
	Source string `gorm:"column:source;type:varchar(20);not null"`

	SourceAddress *string `gorm:"column:source_address;type:varchar(255)"`
	TxHash        *string `gorm:"column:tx_hash;type:varchar(255)"`

	Attachments []byte `gorm:"column:attachments;type:json"`

	Status      string `gorm:"column:status;type:varchar(20);not null;index"`
	SubmittedBy int64  `gorm:"column:submitted_by;type:bigint;not null;index"`

	ReviewedBy *int64     `gorm:"column:reviewed_by;type:bigint;index"`
	ReviewedAt *time.Time `gorm:"column:reviewed_at;type:timestamp"`
	ReviewNote *string    `gorm:"column:review_note;type:text"`

	CompletedAt *time.Time `gorm:"column:completed_at;type:timestamp"`

	CreatedAt *time.Time `gorm:"column:created_at;type:timestamp;index"`
	UpdatedAt *time.Time `gorm:"column:updated_at;type:timestamp"`
	DeletedAt *time.Time `gorm:"column:deleted_at;type:timestamp"`
}

func (VaultAdjustmentModel) TableName() string { return "vault_adjustments" }
