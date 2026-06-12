package model

import "time"

// WalletDepositAddressBalanceModel maps to wallet_deposit_address_balances
// 用户充值地址链上余额表（用于归集管理）
type WalletDepositAddressBalanceModel struct {
	ID               int64  `gorm:"column:id;primaryKey;autoIncrement"`
	DepositAddressID int64  `gorm:"column:deposit_address_id;type:bigint;not null;index"`
	UserID           int64  `gorm:"column:user_id;type:bigint;not null;index"`
	AssetCode        string `gorm:"column:asset_code;type:varchar(32);not null;index"`
	ChainCode        string `gorm:"column:chain_code;type:varchar(32);not null;index"`

	// 余额字段
	Balance       string `gorm:"column:balance;type:varchar(100);not null;default:'0'"`
	BalanceRaw    string `gorm:"column:balance_raw;type:decimal(65,0);not null;default:0;index"` // DECIMAL(65,0) in DB; keep as string to avoid int64 overflow
	BalanceUSD    string `gorm:"column:balance_usd;type:varchar(100);not null;default:'0'"`
	BalanceUSDRaw string `gorm:"column:balance_usd_raw;type:decimal(65,0);not null;default:0;index"` // DECIMAL(65,0) in DB; keep as string to avoid int64 overflow

	// 归集相关
	NeedsSweep         int8       `gorm:"column:needs_sweep;type:tinyint(1);not null;default:0;index"`
	SweepThresholdRaw  string     `gorm:"column:sweep_threshold_raw;type:decimal(65,0);not null;default:0"` // DECIMAL(65,0) in DB; keep as string to avoid int64 overflow
	LastSweepAt        *time.Time `gorm:"column:last_sweep_at;type:timestamp(6)"`
	LastSweepAmountRaw string     `gorm:"column:last_sweep_amount_raw;type:decimal(65,0);default:0"` // DECIMAL(65,0) in DB; keep as string to avoid int64 overflow

	// 同步相关
	LastSyncedAt *time.Time `gorm:"column:last_synced_at;type:timestamp(6);index"`
	SyncStatus   string     `gorm:"column:sync_status;type:varchar(20);not null;default:'unknown';index"`
	SyncError    *string    `gorm:"column:sync_error;type:varchar(512)"`

	// 时间戳
	CreatedAt *time.Time `gorm:"column:created_at;type:timestamp(6)"`
	UpdatedAt *time.Time `gorm:"column:updated_at;type:timestamp(6)"`
	DeletedAt *time.Time `gorm:"column:deleted_at;type:timestamp(6);index"`
}

func (WalletDepositAddressBalanceModel) TableName() string {
	return "wallet_deposit_address_balances"
}
