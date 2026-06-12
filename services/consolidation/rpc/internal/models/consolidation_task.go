package models

import (
	"time"

	"internalwallet/common/model"
)

type ConsolidationTaskStatus int32

const (
	ConsolidationTaskStatusPending         ConsolidationTaskStatus = 1
	ConsolidationTaskStatusNeedEnergy      ConsolidationTaskStatus = 2
	ConsolidationTaskStatusInProgress      ConsolidationTaskStatus = 3
	ConsolidationTaskStatusConfirmed       ConsolidationTaskStatus = 4
	ConsolidationTaskStatusFailed          ConsolidationTaskStatus = 5
	ConsolidationTaskStatusTimeout         ConsolidationTaskStatus = 6
	ConsolidationTaskStatusNeedGas         ConsolidationTaskStatus = 7
	ConsolidationTaskStatusCancelled       ConsolidationTaskStatus = 8
	ConsolidationTaskStatusPermanentFailed ConsolidationTaskStatus = 9
	ConsolidationTaskStatusNeedBandwidth   ConsolidationTaskStatus = 10
)

// ConsolidationTask represents a DB-backed consolidation job.
// NOTE: amount/fee fields are stored as strings in smallest unit for multi-chain compatibility.
type ConsolidationTask struct {
	model.BaseModel

	TaskID        string  `gorm:"column:task_id;size:64;uniqueIndex:uk_consolidation_tasks_task_id;not null" json:"task_id"`
	Chain         string  `gorm:"column:chain;size:20;index:idx_consolidation_tasks_chain_status,priority:1;not null" json:"chain"`
	AssetSymbol   string  `gorm:"column:asset_symbol;size:20;not null" json:"asset_symbol"`
	TokenContract *string `gorm:"column:token_contract;size:100" json:"token_contract,omitempty"`

	FromAddress string `gorm:"column:from_address;size:100;index:idx_consolidation_tasks_from_address;not null" json:"from_address"`
	ToAddress   string `gorm:"column:to_address;size:100;not null" json:"to_address"`

	Amount       string  `gorm:"column:amount;size:64;not null" json:"amount"`
	EstimatedFee *string `gorm:"column:estimated_fee;size:64" json:"estimated_fee,omitempty"`
	ActualFee    *string `gorm:"column:actual_fee;size:64" json:"actual_fee,omitempty"`

	Status       ConsolidationTaskStatus `gorm:"column:status;index:idx_consolidation_tasks_status;index:idx_consolidation_tasks_chain_status,priority:2;not null;default:1" json:"status"`
	TxHash       *string                 `gorm:"column:tx_hash;size:100;index:idx_consolidation_tasks_tx_hash" json:"tx_hash,omitempty"`
	SignedTx     *string                 `gorm:"column:signed_tx;type:text" json:"-"`
	RetryCount   int32                   `gorm:"column:retry_count;not null;default:0" json:"retry_count"`
	BumpCount    int32                   `gorm:"column:bump_count;not null;default:0" json:"bump_count"`
	Version      int64                   `gorm:"column:version;not null;default:0" json:"version"`
	ErrorMessage *string                 `gorm:"column:error_message;type:text" json:"error_message,omitempty"`

	EnergyRentalID     *int64 `gorm:"column:energy_rental_id" json:"energy_rental_id,omitempty"`
	EnergyOrderCount   int32  `gorm:"column:energy_order_count;not null;default:0" json:"energy_order_count"`
	EnergyTotalCostSun int64  `gorm:"column:energy_total_cost_sun;not null;default:0" json:"energy_total_cost_sun"`

	StartedAt     *time.Time `gorm:"column:started_at" json:"started_at,omitempty"`
	ConfirmedAt   *time.Time `gorm:"column:confirmed_at" json:"confirmed_at,omitempty"`
	NextAttemptAt *time.Time `gorm:"column:next_attempt_at;index:idx_consolidation_tasks_next_attempt_at" json:"next_attempt_at,omitempty"`

	ClaimedBy    *string    `gorm:"column:claimed_by;size:64" json:"claimed_by,omitempty"`
	ClaimedUntil *time.Time `gorm:"column:claimed_until;index:idx_consolidation_tasks_claimed_until" json:"claimed_until,omitempty"`

	// Generated column in DB; read-only in GORM.
	ActiveTaskKey *string `gorm:"->;column:active_task_key" json:"active_task_key,omitempty"`
}

func (ConsolidationTask) TableName() string {
	return "consolidation_tasks"
}
