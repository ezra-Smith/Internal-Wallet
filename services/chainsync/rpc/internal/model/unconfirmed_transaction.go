package model

import (
	"internalwallet/common/model"
	"time"
)

// ExecStatus represents on-chain execution status (separate from pipeline Status).
// - unknown: we haven't verified receipt/txInfo yet (or it was temporarily unavailable)
// - confirmed: executed successfully on-chain
// - failed: executed and reverted/failed on-chain
type ExecStatus uint8

const (
	ExecStatusUnknown   ExecStatus = 0
	ExecStatusConfirmed ExecStatus = 1
	ExecStatusFailed    ExecStatus = 2
)

// UnconfirmedTransaction 未确认交易模型
type UnconfirmedTransaction struct {
	model.BaseModel
	TxHash                 string `gorm:"type:varchar(128);not null;uniqueIndex:idx_tx_log" json:"tx_hash"`                 // 交易哈希
	Chain                  string `gorm:"type:varchar(50);not null;index" json:"chain"`                                     // 区块链类型
	BlockNumber            uint64 `gorm:"not null;index" json:"block_number"`                                               // 交易所在区块
	BlockHash              string `gorm:"type:varchar(128);not null" json:"block_hash"`                                     // 区块哈希
	FromAddress            string `gorm:"type:varchar(128);not null;index" json:"from_address"`                             // 发送方地址
	ToAddress              string `gorm:"type:varchar(128);not null;index" json:"to_address"`                               // 接收方地址
	MonitoredAddress       string `gorm:"type:varchar(128);not null;index;uniqueIndex:idx_tx_log" json:"monitored_address"` // 触发监控的地址（按地址维度存储同一事件）
	Source                 string `gorm:"type:varchar(32);not null;index;uniqueIndex:idx_tx_log" json:"source"`             // 地址来源：deposit/company/vault/web3/manual
	Direction              string `gorm:"type:varchar(8);not null;index" json:"direction"`                                  // 方向：in/out
	CounterpartyAddress    string `gorm:"type:varchar(128);not null;index" json:"counterparty_address"`                     // 对手方地址（direction=in -> from；direction=out -> to）
	MonitoredIsInternal    bool   `gorm:"not null;default:false" json:"monitored_is_internal"`                              // 监控地址是否为内部地址
	CounterpartyIsInternal bool   `gorm:"not null;default:false" json:"counterparty_is_internal"`                           // 对手方是否为内部地址
	CounterpartySourceBits uint8  `gorm:"type:tinyint unsigned;not null;default:0" json:"counterparty_source_bits"`         // 对手方来源 bitset（用于下游判断 internal/internal）
	Value                  string `gorm:"type:varchar(128);not null" json:"value"`                                          // 交易金额
	GasPrice               string `gorm:"type:varchar(128)" json:"gas_price"`                                               // Gas价格
	GasUsed                uint64 `gorm:"type:bigint" json:"gas_used"`                                                      // Gas使用量
	GasFee                 string `gorm:"type:varchar(128)" json:"gas_fee"`                                                 // Gas费用
	EnergyUsed             uint64 `gorm:"type:bigint;default:0" json:"energy_used"`                                         // TRON能量消耗
	BandwidthUsed          uint64 `gorm:"type:bigint;default:0" json:"bandwidth_used"`                                      // TRON带宽消耗
	TransactionIndex       uint32 `gorm:"not null" json:"transaction_index"`                                                // 交易在区块中的索引
	BlockTimestamp         int64  `gorm:"not null" json:"block_timestamp"`                                                  // 区块时间戳
	LogIndex               int32  `gorm:"not null;default:0;uniqueIndex:idx_tx_log" json:"log_index"`                       // 日志索引，用于区分同一交易的多个Transfer事件
	TransactionType        string `gorm:"type:varchar(20);not null;default:'native';index" json:"transaction_type"`         // 交易类型: native(主币) | token(代币)
	TokenAddress           string `gorm:"type:varchar(128);index" json:"token_address,omitempty"`                           // 代币合约地址（仅代币交易）
	TokenName              string `gorm:"type:varchar(100)" json:"token_name,omitempty"`                                    // 代币名称（仅代币交易）
	TokenSymbol            string `gorm:"type:varchar(50)" json:"token_symbol,omitempty"`                                   // 代币符号（仅代币交易）
	TokenDecimals          *uint8 `gorm:"type:tinyint unsigned" json:"token_decimals,omitempty"`                            // 代币精度（仅代币交易）
	TokenAmount            string `gorm:"type:varchar(128)" json:"token_amount,omitempty"`                                  // 代币原始数量（仅代币交易）

	// 链上执行状态（与 pipeline Status 分离）
	ExecStatus       ExecStatus `gorm:"type:tinyint unsigned;not null;default:0;index" json:"exec_status"` // 0=unknown, 1=confirmed, 2=failed
	ExecCheckedAt    *time.Time `gorm:"type:datetime" json:"exec_checked_at,omitempty"`                    // 上次检查链上回执时间
	ExecNextCheckAt  *time.Time `gorm:"type:datetime;index" json:"exec_next_check_at,omitempty"`           // 下次允许检查时间（节流）
	ExecErrorMessage string     `gorm:"type:text" json:"exec_error_message"`                               // 检查失败或失败原因（best-effort）

	Status                uint8      `gorm:"not null;default:0" json:"status"`                  // 状态：0-待确认，1-已发送Kafka，2-确认失败
	Confirmations         int32      `gorm:"not null;default:0" json:"confirmations"`           // 当前确认数
	RequiredConfirmations int32      `gorm:"not null;default:12" json:"required_confirmations"` // 需要的确认数
	MessageID             string     `gorm:"type:varchar(128)" json:"message_id"`               // 消息ID
	MessageTopic          string     `gorm:"type:varchar(128)" json:"message_topic"`            // 消息主题
	SentAt                *time.Time `gorm:"type:datetime" json:"sent_at"`                      // 发送到消息队列的时间
	//ConfirmedAt           *time.Time     `gorm:"type:datetime" json:"confirmed_at"`                         // 确认时间
	ErrorMessage string `gorm:"type:text" json:"error_message"`        // 错误信息
	RetryCount   int32  `gorm:"not null;default:0" json:"retry_count"` // 重试次数
	MaxRetries   int32  `gorm:"not null;default:3" json:"max_retries"` // 最大重试次数
}

// TableName 指定表名
func (UnconfirmedTransaction) TableName() string {
	return "unconfirmed_transaction"
}

// IsConfirmed 检查是否已确认
func (ut *UnconfirmedTransaction) IsConfirmed() bool {
	return ut.Status == 1
}

// IsPending 检查是否待确认
func (ut *UnconfirmedTransaction) IsPending() bool {
	return ut.Status == 0
}

// IsFailed 检查是否确认失败
func (ut *UnconfirmedTransaction) IsFailed() bool {
	return ut.Status == 2
}

// HasEnoughConfirmations 检查是否有足够的确认数
func (ut *UnconfirmedTransaction) HasEnoughConfirmations(currentBlock uint64) bool {
	if ut.BlockNumber == 0 {
		return false
	}

	// Guard against reorg / stale currentBlock causing uint underflow.
	if currentBlock < ut.BlockNumber {
		ut.Confirmations = 0
		return false
	}

	confirmations := int32(currentBlock - ut.BlockNumber)
	ut.Confirmations = confirmations
	return confirmations >= ut.RequiredConfirmations
}

// CanRetry 检查是否可以重试
func (ut *UnconfirmedTransaction) CanRetry() bool {
	return ut.Status == 2 && ut.RetryCount < ut.MaxRetries
}

// IncrementRetry 增加重试次数
func (ut *UnconfirmedTransaction) IncrementRetry() {
	ut.RetryCount++
	ut.Status = 0 // 重置为待确认状态
	ut.ErrorMessage = ""
}

// IsNativeTransaction 检查是否为主币交易
func (ut *UnconfirmedTransaction) IsNativeTransaction() bool {
	return ut.TransactionType == "native" || ut.TransactionType == ""
}

// IsTokenTransaction 检查是否为代币交易
func (ut *UnconfirmedTransaction) IsTokenTransaction() bool {
	return ut.TransactionType == "token"
}
