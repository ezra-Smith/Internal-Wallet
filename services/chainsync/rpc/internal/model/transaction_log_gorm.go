package model

import (
	"database/sql/driver"
	"encoding/json"
	commonModel "internalwallet/common/model"
)

// TransactionLogGorm 交易日志 GORM 模型
type TransactionLogGorm struct {
	commonModel.BaseModel

	// 区块链类型
	Chain string `gorm:"column:chain;size:32;not null;index:idx_chain_tx_log,priority:1" json:"chain"`

	// 交易哈希
	TransactionHash string `gorm:"column:transaction_hash;size:128;not null;index:idx_chain_tx_log,priority:2" json:"transaction_hash"`

	// 日志索引
	LogIndex int `gorm:"column:log_index;not null;index:idx_chain_tx_log,priority:3" json:"log_index"`

	// 合约地址
	Address string `gorm:"column:address;size:128;not null;index:idx_address" json:"address"`

	// 事件主题（数组，使用JSON存储）
	Topics Topics `gorm:"column:topics;type:json" json:"topics"`

	// 事件数据
	Data string `gorm:"column:data;type:longtext" json:"data"`

	// 区块号
	BlockNumber uint64 `gorm:"column:block_number;not null;index:idx_block_number" json:"block_number"`

	// 区块哈希
	BlockHash string `gorm:"column:block_hash;size:128;not null" json:"block_hash"`
}

// Topics 事件主题数组
type Topics []string

// Value 实现 driver.Valuer 接口，用于数据库存储
func (t Topics) Value() (driver.Value, error) {
	if t == nil {
		return nil, nil
	}
	return json.Marshal(t)
}

// Scan 实现 sql.Scanner 接口，用于数据库读取
func (t *Topics) Scan(value interface{}) error {
	if value == nil {
		*t = Topics{}
		return nil
	}

	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, t)
	case string:
		return json.Unmarshal([]byte(v), t)
	default:
		return nil
	}
}

// TableName 指定表名
func (TransactionLogGorm) TableName() string {
	return "transaction_logs"
}
