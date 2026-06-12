package model

import (
	"time"
)

// WalletDepositAddressBalanceChangeModel maps to wallet_deposit_address_balance_changes
// 充值地址余额变动记录表（审计日志）
type WalletDepositAddressBalanceChangeModel struct {
	ID int64 `gorm:"column:id;primaryKey;autoIncrement"`

	// 关键关联字段
	DepositAddressID int64  `gorm:"column:deposit_address_id;type:bigint;not null;index"`
	UserID           int64  `gorm:"column:user_id;type:bigint;not null;index"`
	AssetCode        string `gorm:"column:asset_code;type:varchar(32);not null;index"`
	ChainCode        string `gorm:"column:chain_code;type:varchar(32);not null;index"`

	// 变动类型和原因
	ChangeType   string  `gorm:"column:change_type;type:varchar(32);not null;index"`
	ChangeReason *string `gorm:"column:change_reason;type:varchar(255)"`

	// 余额变动数据（最小单位）
	// DECIMAL(65,0) in DB; keep as string to avoid int64 overflow
	PreviousBalanceRaw string `gorm:"column:previous_balance_raw;type:decimal(65,0);not null;default:0"`
	CurrentBalanceRaw  string `gorm:"column:current_balance_raw;type:decimal(65,0);not null;default:0"`
	ChangeAmountRaw    string `gorm:"column:change_amount_raw;type:decimal(65,0);not null"`

	// 价值变动（USD，分）
	// DECIMAL(65,0) in DB; keep as string to avoid int64 overflow
	PreviousBalanceUSDRaw string `gorm:"column:previous_balance_usd_raw;type:decimal(65,0);not null;default:0"`
	CurrentBalanceUSDRaw  string `gorm:"column:current_balance_usd_raw;type:decimal(65,0);not null;default:0"`
	ChangeAmountUSDRaw    string `gorm:"column:change_amount_usd_raw;type:decimal(65,0);not null;default:0"`

	// 转账地址（必填）
	FromAddress string `gorm:"column:from_address;type:varchar(255);not null"`
	ToAddress   string `gorm:"column:to_address;type:varchar(255);not null"`

	// 交易哈希（必填，唯一标识）
	TxHash string `gorm:"column:tx_hash;type:varchar(255);not null;uniqueIndex:uk_tx_hash"`

	// 链上事件信息（必填，所有记录都来自 Chainsync）
	BlockNumber       int64  `gorm:"column:block_number;type:bigint;not null"`
	BlockHash         string `gorm:"column:block_hash;type:varchar(255);not null"`
	TxIndex           int32  `gorm:"column:tx_index;type:int;not null"`
	LogIndex          *int32 `gorm:"column:log_index;type:int"` // 仅 ERC20 转账有值
	BlockTimestamp    int64  `gorm:"column:block_timestamp;type:bigint;not null"`
	ConfirmationCount int32  `gorm:"column:confirmation_count;type:int;not null;default:0"`

	// 归集相关
	SweepTaskNo *string `gorm:"column:sweep_task_no;type:varchar(64)"`

	// 状态
	Status string `gorm:"column:status;type:varchar(32);not null;default:'confirmed'"`
	Source string `gorm:"column:source;type:varchar(32);not null;default:'chainsync'"`

	// 时间戳
	CreatedAt time.Time `gorm:"column:created_at;type:timestamp(6);not null"`
}

func (WalletDepositAddressBalanceChangeModel) TableName() string {
	return "wallet_deposit_address_balance_changes"
}

// BalanceChangeType 变动类型常量
type BalanceChangeType string

const (
	ChangeTypeDeposit    BalanceChangeType = "deposit"    // 充值
	ChangeTypeSweep      BalanceChangeType = "sweep"      // 归集
	ChangeTypeTransfer   BalanceChangeType = "transfer"   // 转账
	ChangeTypeAdjustment BalanceChangeType = "adjustment" // 调整
	ChangeTypeRollback   BalanceChangeType = "rollback"   // 回滚
	ChangeTypeOther      BalanceChangeType = "other"      // 其他
)

// BalanceChangeStatus 状态常量
type BalanceChangeStatus string

const (
	StatusConfirmed  BalanceChangeStatus = "confirmed"   // 已确认（默认）
	StatusRolledBack BalanceChangeStatus = "rolled_back" // 已回滚（链重组）
)

// BalanceChangeSource 数据来源常量
type BalanceChangeSource string

const (
	SourceChainSync BalanceChangeSource = "chainsync" // chainsync 服务（所有记录统一来源）
)
