package model

import (
	commonModel "internalwallet/common/model"
)

// BlockGorm 区块信息 GORM 模型
// 继承 BaseModel，自动拥有 ID, CreatedAt, UpdatedAt, DeletedAt 等字段
type BlockGorm struct {
	commonModel.BaseModel

	// 区块链ID：ethereum（1）, bsc(56), tron(728126428)
	ChainID int64 `gorm:"column:chain_id;not null;index:idx_chain_block,priority:1" json:"chain_id"`

	// 区块号
	BlockNumber uint64 `gorm:"column:block_number;not null;index:idx_chain_block,priority:2;index:idx_block_number" json:"block_number"`

	// 区块哈希
	BlockHash string `gorm:"column:block_hash;size:128;not null;index:idx_block_hash" json:"block_hash"`

	// 父区块哈希
	ParentHash string `gorm:"column:parent_hash;size:128" json:"parent_hash"`

	// 区块时间戳
	Timestamp int64 `gorm:"column:timestamp;not null;index" json:"timestamp"`

	// 交易数量
	TransactionCount int `gorm:"column:transaction_count;not null;default:0" json:"transaction_count"`

	// 矿工地址（TRON没有此字段）
	Miner string `gorm:"column:miner;size:64" json:"miner"`

	// 难度值（TRON没有此字段）
	Difficulty string `gorm:"column:difficulty;size:64" json:"difficulty"`

	// Gas限制（TRON没有此字段）
	GasLimit uint64 `gorm:"column:gas_limit;default:0" json:"gas_limit"`

	// Gas使用量（TRON没有此字段）
	GasUsed uint64 `gorm:"column:gas_used;default:0" json:"gas_used"`

	// 区块大小（字节）
	Size uint64 `gorm:"column:size;default:0" json:"size"`

	// 同步状态：pending, synced, failed
	SyncStatus string `gorm:"column:sync_status;size:32;not null;default:'synced';index:idx_sync_status" json:"sync_status"`
}

// TableName 指定表名
func (BlockGorm) TableName() string {
	return "blocks"
}
