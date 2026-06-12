package models

import (
	"time"

	"internalwallet/common/model"
)

type ConsolidationTopUpStatus int32

const (
	ConsolidationTopUpStatusPending   ConsolidationTopUpStatus = 1
	ConsolidationTopUpStatusSent      ConsolidationTopUpStatus = 2
	ConsolidationTopUpStatusConfirmed ConsolidationTopUpStatus = 3
	ConsolidationTopUpStatusFailed    ConsolidationTopUpStatus = 4
)

// ConsolidationTopUpRecord tracks gas/activation funding from company hot wallets to user deposit addresses.
// Amount fields are stored as strings in smallest unit for multi-chain compatibility.
type ConsolidationTopUpRecord struct {
	model.BaseModel

	TaskID      string `gorm:"column:task_id;size:64;index:idx_consolidation_topup_records_task_id;not null" json:"task_id"`
	Chain       string `gorm:"column:chain;size:20;index:idx_consolidation_topup_records_to_address_chain,priority:2;not null" json:"chain"`
	AssetSymbol string `gorm:"column:asset_symbol;size:20;not null" json:"asset_symbol"`

	FromAddress string `gorm:"column:from_address;size:100;not null" json:"from_address"`
	ToAddress   string `gorm:"column:to_address;size:100;index:idx_consolidation_topup_records_to_address_chain,priority:1;not null" json:"to_address"`

	Amount  string `gorm:"column:amount;size:40;not null" json:"amount"`
	Purpose string `gorm:"column:purpose;size:20;not null" json:"purpose"`

	Status       ConsolidationTopUpStatus `gorm:"column:status;index:idx_consolidation_topup_records_status;not null;default:1" json:"status"`
	TxHash       *string                  `gorm:"column:tx_hash;size:100;index:idx_consolidation_topup_records_tx_hash" json:"tx_hash,omitempty"`
	SignedTx     *string                  `gorm:"column:signed_tx;type:text" json:"-"`
	RetryCount   int32                    `gorm:"column:retry_count;not null;default:0" json:"retry_count"`
	ErrorMessage *string                  `gorm:"column:error_message;type:text" json:"error_message,omitempty"`
	ConfirmedAt  *time.Time               `gorm:"column:confirmed_at" json:"confirmed_at,omitempty"`

	// Generated column in DB; read-only in GORM.
	ActiveTopUpKey *string `gorm:"->;column:active_topup_key" json:"active_topup_key,omitempty"`
}

func (ConsolidationTopUpRecord) TableName() string {
	return "consolidation_topup_records"
}
