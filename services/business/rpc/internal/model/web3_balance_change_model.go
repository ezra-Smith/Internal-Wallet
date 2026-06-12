package model

import (
	commonmodel "internalwallet/common/model"
	"time"
)

// Web3BalanceChangeModel maps to web3_balance_changes (event-level).
// One record represents ONE balance-change event for ONE address.
type Web3BalanceChangeModel struct {
	commonmodel.BaseModel

	// OldID links this record to a previous synthetic/broadcast placeholder record id.
	// It is used to preserve detail lookups when the placeholder is soft-deleted after confirmation.
	OldID *int64 `gorm:"column:old_id;type:bigint;index:idx_web3_balance_changes_old_id"`

	Web3UserID  int64  `gorm:"column:web3_user_id;type:bigint;not null;default:0;index:idx_web3_balance_changes_web3_user_id"`
	UserAddress string `gorm:"column:user_address;type:varchar(255);not null;uniqueIndex:uk_web3_balance_changes_tx_event_addr,priority:3;index:idx_web3_balance_changes_user_address"`

	ChainCode string `gorm:"column:chain_code;type:varchar(20);not null;index:idx_web3_balance_changes_chain_code"`
	ChainID   int64  `gorm:"column:chain_id;type:bigint;not null;default:0;index:idx_web3_balance_changes_chain_id"`

	TxHash     string `gorm:"column:tx_hash;type:varchar(255);not null;uniqueIndex:uk_web3_balance_changes_tx_event_addr,priority:1;index:idx_web3_balance_changes_tx_hash"`
	EventIndex int32  `gorm:"column:event_index;type:int;not null;default:0;uniqueIndex:uk_web3_balance_changes_tx_event_addr,priority:2;index:idx_web3_balance_changes_event_index"`

	BlockNumber *int64     `gorm:"column:block_number;type:bigint"`
	BlockTime   *time.Time `gorm:"column:block_time;type:datetime(3);index:idx_web3_balance_changes_block_time"`

	TxType        string  `gorm:"column:tx_type;type:varchar(50);not null;index:idx_web3_balance_changes_tx_type"`
	Direction     string  `gorm:"column:direction;type:varchar(20);not null;index:idx_web3_balance_changes_direction"`
	AssetCode     string  `gorm:"column:asset_code;type:varchar(50);not null;index:idx_web3_balance_changes_asset_code"`
	Amount        string  `gorm:"column:amount;type:varchar(100);not null"`
	AmountRaw     *string `gorm:"column:amount_raw;type:varchar(128)"`
	TokenAddress  *string `gorm:"column:token_address;type:varchar(128)"`
	TokenDecimals *int32  `gorm:"column:token_decimals;type:int"`

	FromAddress *string `gorm:"column:from_address;type:varchar(255)"`
	ToAddress   *string `gorm:"column:to_address;type:varchar(255)"`

	Status        string `gorm:"column:status;type:varchar(50);not null;default:'pending';index:idx_web3_balance_changes_status"`
	Confirmations int32  `gorm:"column:confirmations;type:int;not null;default:0"`

	Fee      *string `gorm:"column:fee;type:varchar(100)"`
	FeeAsset *string `gorm:"column:fee_asset;type:varchar(50)"`

	RawData []byte `gorm:"column:raw_data;type:json"`
}

func (Web3BalanceChangeModel) TableName() string { return "web3_balance_changes" }

const (
	// Web3BalanceChangeBroadcastEventIndex is used for synthetic records created at broadcast time.
	// Confirmed on-chain events use non-negative event_index values from chainsync.
	Web3BalanceChangeBroadcastEventIndex int32 = -1
)
