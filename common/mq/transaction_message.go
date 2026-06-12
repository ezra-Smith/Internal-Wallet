package mq

import "time"

// TransactionConfirmMessage 交易确认消息结构体
// 用于跨服务传递已确认的交易信息（支持主币和代币交易）
type TransactionConfirmMessage struct {
	// Envelope
	Version int32  `json:"version"` // schema version
	EventID string `json:"event_id"`

	// Routing (single-source event; fan-out happens in chainsync)
	Source                 AddressMonitorSource   `json:"source"`                // deposit/company/vault/web3/manual/unknown
	Direction              string                 `json:"direction"`             // in/out
	CounterpartyAddress    string                 `json:"counterparty_address"`  // counterparty for this monitored address + direction
	MonitoredIsInternal    bool                   `json:"monitored_is_internal"` // derived from deposit/company/vault membership
	CounterpartyIsInternal bool                   `json:"counterparty_is_internal"`
	CounterpartySources    []AddressMonitorSource `json:"counterparty_sources,omitempty"`
	CounterpartySourceBits uint8                  `json:"counterparty_source_bits,omitempty"`

	// 基础字段
	TxHash                string    `json:"tx_hash"`                // 交易哈希
	Chain                 string    `json:"chain"`                  // 区块链类型
	ChainId               int64     `json:"chain_id"`               // 链ID（EIP-155/系统链ID；由 chainsync 填充，便于下游精确匹配地址所属链）
	BlockNumber           uint64    `json:"block_number"`           // 交易所在区块
	BlockHash             string    `json:"block_hash"`             // 区块哈希
	FromAddress           string    `json:"from_address"`           // 发送方地址
	ToAddress             string    `json:"to_address"`             // 接收方地址
	MonitoredAddress      string    `json:"monitored_address"`      // 触发监控的地址
	Value                 string    `json:"value"`                  // 交易金额
	GasPrice              string    `json:"gas_price"`              // Gas价格
	GasUsed               uint64    `json:"gas_used"`               // Gas使用量
	GasFee                string    `json:"gas_fee"`                // Gas费用
	TransactionIndex      uint32    `json:"transaction_index"`      // 交易在区块中的索引
	BlockTimestamp        int64     `json:"block_timestamp"`        // 区块时间戳
	Confirmations         int32     `json:"confirmations"`          // 当前确认数
	RequiredConfirmations int32     `json:"required_confirmations"` // 需要的确认数
	CreatedAt             time.Time `json:"created_at"`             // 创建时间

	// 交易类型字段（用于区分主币和代币交易）
	TransactionType string `json:"transaction_type"` // 交易类型: "native" | "token"
	LogIndex        int32  `json:"log_index"`        // 事件索引（log_index / internal trace index）

	// 代币相关字段（仅当 TransactionType="token" 时有值）
	TokenAddress  string `json:"token_address,omitempty"`  // 代币合约地址
	TokenName     string `json:"token_name,omitempty"`     // 代币名称
	TokenSymbol   string `json:"token_symbol,omitempty"`   // 代币符号
	TokenDecimals uint8  `json:"token_decimals,omitempty"` // 代币精度
	TokenAmount   string `json:"token_amount,omitempty"`   // 代币原始数量
}

// IsNativeTransaction 判断是否为主币交易
func (m *TransactionConfirmMessage) IsNativeTransaction() bool {
	return m.TransactionType == "native" || m.TransactionType == ""
}

// IsTokenTransaction 判断是否为代币交易
func (m *TransactionConfirmMessage) IsTokenTransaction() bool {
	return m.TransactionType == "token"
}
