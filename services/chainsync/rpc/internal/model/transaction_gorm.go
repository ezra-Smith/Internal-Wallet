package model

import (
	commonModel "internalwallet/common/model"
)

// TransactionGorm 交易信息 GORM 模型
type TransactionGorm struct {
	commonModel.BaseModel

	// 区块链类型
	Chain string `gorm:"column:chain;size:32;not null;index:idx_chain_tx,priority:1" json:"chain"`

	// 交易哈希
	TxHash string `gorm:"column:tx_hash;size:128;not null;index:idx_chain_tx,priority:2;index:idx_tx_hash" json:"tx_hash"`

	// 区块号
	BlockNumber uint64 `gorm:"column:block_number;not null;index:idx_block_number" json:"block_number"`

	// 区块哈希
	BlockHash string `gorm:"column:block_hash;size:128;not null" json:"block_hash"`

	// 交易在区块中的索引
	TransactionIndex int `gorm:"column:transaction_index;not null;default:0" json:"transaction_index"`

	// 发送方地址
	FromAddress string `gorm:"column:from_address;size:128;not null;index:idx_from_address" json:"from_address"`

	// 接收方地址
	ToAddress string `gorm:"column:to_address;size:128;index:idx_to_address" json:"to_address"`

	// 交易金额（最小单位）
	Value string `gorm:"column:value;size:64;not null;default:'0'" json:"value"`

	// Gas价格
	GasPrice string `gorm:"column:gas_price;size:64" json:"gas_price"`

	// Gas使用量
	GasUsed string `gorm:"column:gas_used;size:64;default:'0'" json:"gas_used"`

	// Gas费用
	GasFee string `gorm:"column:gas_fee;size:64" json:"gas_fee"`

	// Nonce
	Nonce string `gorm:"column:nonce;size:64" json:"nonce"`

	// 输入数据（使用TEXT类型，在GORM中会自动处理）
	InputData string `gorm:"column:input_data;type:longtext" json:"input_data"`

	// 确认数
	Confirmations uint64 `gorm:"column:confirmations;default:0;index:idx_confirmations" json:"confirmations"`

	// 交易状态：pending, confirmed, failed, reverted
	Status string `gorm:"column:status;size:32;not null;default:'pending';index:idx_status" json:"status"`

	// 合约地址（合约创建时）
	ContractAddress string `gorm:"column:contract_address;size:128" json:"contract_address"`

	// 错误信息（失败时）
	ErrorMessage string `gorm:"column:error_message;type:text" json:"error_message"`
}

// TableName 指定表名
func (TransactionGorm) TableName() string {
	return "transactions"
}
