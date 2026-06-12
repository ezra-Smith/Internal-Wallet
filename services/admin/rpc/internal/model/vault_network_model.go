package model

import "time"

// VaultNetworkModel maps to vault_networks (vault address & sync status per chain).
type VaultNetworkModel struct {
	ID           int64   `gorm:"column:id;primaryKey;autoIncrement"`
	ChainID      int64   `gorm:"column:chain_id;type:bigint;not null;index"`
	Network      string  `gorm:"column:network;type:varchar(32);not null;index"`
	ChainType    string  `gorm:"column:chain_type;type:varchar(50);not null;index"`
	VaultAddress *string `gorm:"column:vault_address;type:varchar(255)"`

	Status     string     `gorm:"column:status;type:varchar(20);not null;default:'active';index"`
	SyncStatus string     `gorm:"column:sync_status;type:varchar(20);not null;default:'unknown';index"`
	LastSyncAt *time.Time `gorm:"column:last_sync_at;type:timestamp"`

	LastBlockSynced *int64 `gorm:"column:last_block_synced;type:bigint"`
	CurrentBlock    *int64 `gorm:"column:current_block;type:bigint"`

	CreatedAt *time.Time `gorm:"column:created_at;type:timestamp"`
	UpdatedAt *time.Time `gorm:"column:updated_at;type:timestamp"`
	DeletedAt *time.Time `gorm:"column:deleted_at;type:timestamp"`
}

func (VaultNetworkModel) TableName() string { return "vault_networks" }
