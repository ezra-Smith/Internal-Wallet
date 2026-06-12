package model

import (
	"time"

	"gorm.io/gorm"
)

// AcctLedgerPostingModel maps to `acct_ledger_postings`.
type AcctLedgerPostingModel struct {
	ID        int64  `gorm:"column:id;primaryKey;autoIncrement"`
	TxID      int64  `gorm:"column:tx_id;type:bigint;not null;index;uniqueIndex:uk_acct_ledger_postings_tx_seq,priority:1"`
	Seq       int32  `gorm:"column:seq;type:int;not null;uniqueIndex:uk_acct_ledger_postings_tx_seq,priority:2"`
	AssetCode string `gorm:"column:asset_code;type:varchar(32);not null;index"`
	AccountID int64  `gorm:"column:account_id;type:bigint;not null;index"`
	Bucket    string `gorm:"column:bucket;type:varchar(16);not null;index"`

	// debit_raw / credit_raw are DECIMAL(65,0) in DB; keep as string.
	DebitRaw  string `gorm:"column:debit_raw;type:decimal(65,0);not null;default:0"`
	CreditRaw string `gorm:"column:credit_raw;type:decimal(65,0);not null;default:0"`

	CreatedAt time.Time      `gorm:"column:created_at;type:datetime;autoCreateTime"`
	UpdatedAt time.Time      `gorm:"column:updated_at;type:datetime;autoUpdateTime"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (AcctLedgerPostingModel) TableName() string { return "acct_ledger_postings" }
