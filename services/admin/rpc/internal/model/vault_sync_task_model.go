package model

import "time"

// VaultSyncTaskModel maps to vault_sync_tasks (manual sync history).
type VaultSyncTaskModel struct {
	ID        int64  `gorm:"column:id;primaryKey;autoIncrement"`
	NetworkID int64  `gorm:"column:network_id;type:bigint;not null;index"`
	ChainID   int64  `gorm:"column:chain_id;type:bigint;not null;index"`
	Network   string `gorm:"column:network;type:varchar(32);not null;index"`

	Status       string     `gorm:"column:status;type:varchar(20);not null;index"`
	StartedAt    *time.Time `gorm:"column:started_at;type:timestamp"`
	FinishedAt   *time.Time `gorm:"column:finished_at;type:timestamp"`
	ErrorMessage *string    `gorm:"column:error_message;type:varchar(512)"`

	CreatedBy int64 `gorm:"column:created_by;type:bigint;not null;default:0;index"`

	CreatedAt *time.Time `gorm:"column:created_at;type:timestamp;index"`
	UpdatedAt *time.Time `gorm:"column:updated_at;type:timestamp"`
	DeletedAt *time.Time `gorm:"column:deleted_at;type:timestamp"`
}

func (VaultSyncTaskModel) TableName() string { return "vault_sync_tasks" }
