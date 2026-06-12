package model

import (
	commonModel "internalwallet/common/model"
)

// AcctUserTransactionRecordModel maps to `acct_user_transaction_records`.
// It is an Accounting-owned read model used by Business APIs for user-facing
// deposit/withdraw record lists and details.
// Note: Internal transfers are recorded as withdraw (type=2) for sender and deposit (type=1) for receiver.
type AcctUserTransactionRecordModel struct {
	commonModel.BaseModel

	UserID    int64  `gorm:"column:user_id;type:bigint;not null;index"`
	TxType    int32  `gorm:"column:tx_type;type:int;not null;index"` // 1=deposit, 2=withdraw（包含内部转账转出方）
	AssetCode string `gorm:"column:asset_code;type:varchar(32);not null;index"`
	ChainCode string `gorm:"column:chain_code;type:varchar(32);not null;default:'';index"`

	// Decimal strings in asset units.
	AmountDecimal string `gorm:"column:amount_decimal;type:decimal(65,30);not null;default:0"`
	FeeDecimal    string `gorm:"column:fee_decimal;type:decimal(65,30);not null;default:0"`

	Status string `gorm:"column:status;type:varchar(20);not null;default:'pending';index"`
	Memo   string `gorm:"column:memo;type:varchar(255);not null;default:''"`

	FromAddress string `gorm:"column:from_address;type:varchar(255);not null;default:''"`
	ToAddress   string `gorm:"column:to_address;type:varchar(255);not null;default:''"`
	TxHash      string `gorm:"column:tx_hash;type:varchar(255);not null;default:'';index"`

	FreezeLedgerTxID  *int64 `gorm:"column:freeze_ledger_tx_id;type:bigint;default:null"`
	SettleLedgerTxID  *int64 `gorm:"column:settle_ledger_tx_id;type:bigint;default:null"`
	ConfirmLedgerTxID *int64 `gorm:"column:confirm_ledger_tx_id;type:bigint;default:null"`

	BizRef         string `gorm:"column:biz_ref;type:varchar(128);not null;default:''"`
	IdempotencyKey string `gorm:"column:idempotency_key;type:varchar(128);not null;default:''"`
}

func (AcctUserTransactionRecordModel) TableName() string { return "acct_user_transaction_records" }
