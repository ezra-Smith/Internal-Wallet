package model

import (
	commonModel "internalwallet/common/model"
)

// SyncTaskGorm 同步任务 GORM 模型
type SyncTaskGorm struct {
	commonModel.BaseModel

	// 同步任��ID
	SyncID string `gorm:"column:sync_id;size:128;not null;uniqueIndex:idx_sync_id" json:"sync_id"`

	// 区块链类型
	Chain string `gorm:"column:chain;size:32;not null;index:idx_chain_status,priority:1" json:"chain"`

	// 同步状态：stopped, syncing, caught_up, error, paused
	Status string `gorm:"column:status;size:32;not null;default:'stopped';index:idx_chain_status,priority:2;index:idx_status" json:"status"`

	// 起始区块号
	StartBlock uint64 `gorm:"column:start_block;not null;default:0" json:"start_block"`

	// 结束区块号，0表示持续同步
	EndBlock uint64 `gorm:"column:end_block;not null;default:0" json:"end_block"`

	// 当前区块号
	CurrentBlock uint64 `gorm:"column:current_block;not null;default:0" json:"current_block"`

	// 最新区块号
	LatestBlock uint64 `gorm:"column:latest_block;not null;default:0" json:"latest_block"`

	// 同步速度（块/秒）
	BlocksPerSecond float64 `gorm:"column:blocks_per_second;type:decimal(10,2);default:0" json:"blocks_per_second"`

	// 预计剩余时间（秒）
	EstimatedRemainingSeconds int64 `gorm:"column:estimated_remaining_seconds;default:0" json:"estimated_remaining_seconds"`

	// 最后错误信息
	LastError string `gorm:"column:last_error;type:text" json:"last_error"`

	// 开始时间
	StartTime *int64 `gorm:"column:start_time" json:"start_time"`

	// 结束时间
	EndTime *int64 `gorm:"column:end_time" json:"end_time"`

	// 最后更新时间
	LastUpdateTime *int64 `gorm:"column:last_update_time;index:idx_last_update" json:"last_update_time"`
}

// TableName 指定表名
func (SyncTaskGorm) TableName() string {
	return "sync_tasks"
}
