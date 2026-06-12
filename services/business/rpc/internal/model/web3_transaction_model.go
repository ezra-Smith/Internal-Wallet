package model

import "time"

// Web3TransactionModel 映射到 web3_transactions 表（Web3 用户的链上交易记录）
type Web3TransactionModel struct {
	ID          int64  `gorm:"column:id;primaryKey;autoIncrement"`
	Web3UserID  int64  `gorm:"column:web3_user_id;type:bigint;not null;index"`
	UserAddress string `gorm:"column:user_address;type:varchar(255);not null;index"` // 用户的地址

	// 链信息
	Network string `gorm:"column:network;type:varchar(50);not null;index"` // Ethereum/Bitcoin/Tron
	ChainID int64  `gorm:"column:chain_id;type:bigint;not null;default:0;index"`

	// 交易基本信息
	TxHash      string     `gorm:"column:tx_hash;type:varchar(255);not null;uniqueIndex"` // 交易哈希
	BlockNumber int64      `gorm:"column:block_number;type:bigint;index"`                 // 区块高度
	BlockTime   *time.Time `gorm:"column:block_time;type:timestamp;index"`                // 区块时间

	// 交易类型和方向
	TxType    string `gorm:"column:tx_type;type:varchar(50);not null;index"`   // receive/send/swap/contract_call
	Direction string `gorm:"column:direction;type:varchar(20);not null;index"` // in/out

	// 金额信息
	AssetCode string `gorm:"column:asset_code;type:varchar(50);not null;index"` // BTC/ETH/USDT
	Amount    string `gorm:"column:amount;type:varchar(100);not null"`          // 交易金额
	AmountUSD string `gorm:"column:amount_usd;type:varchar(100)"`               // USD 估值

	// 地址信息
	FromAddress string `gorm:"column:from_address;type:varchar(255);index"` // 发送方地址
	ToAddress   string `gorm:"column:to_address;type:varchar(255);index"`   // 接收方地址

	// 交易状态
	Status        string `gorm:"column:status;type:varchar(50);not null;default:'pending';index"` // pending/confirmed/failed
	Confirmations int32  `gorm:"column:confirmations;type:int;not null;default:0"`                // 当前确认数

	// 手续费
	Fee      string `gorm:"column:fee;type:varchar(100)"`      // 手续费金额
	FeeAsset string `gorm:"column:fee_asset;type:varchar(50)"` // 手续费币种（通常是原生币）

	// 原始数据（存储完整的链上数据，方便后续扩展解析）
	RawData []byte `gorm:"column:raw_data;type:json"` // 原始交易数据 JSON

	// 时间戳
	CreatedAt *time.Time `gorm:"column:created_at;type:timestamp;index"`
	UpdatedAt *time.Time `gorm:"column:updated_at;type:timestamp"`
}

func (Web3TransactionModel) TableName() string { return "web3_transactions" }

// 交易类型常量
const (
	Web3TxTypeReceive      = "receive"       // 普通接收
	Web3TxTypeSend         = "send"          // 普通发送
	Web3TxTypeSwap         = "swap"          // DEX 交互
	Web3TxTypeContractCall = "contract_call" // 合约调用
	Web3TxTypeApprove      = "approve"       // 代币授权
)

// 交易方向常量
const (
	Web3TxDirectionIn  = "in"  // 收入
	Web3TxDirectionOut = "out" // 支出
)

// 交易状态常量
const (
	Web3TxStatusPending   = "pending"   // 待确认
	Web3TxStatusConfirmed = "confirmed" // 已确认
	Web3TxStatusFailed    = "failed"    // 失败
)
