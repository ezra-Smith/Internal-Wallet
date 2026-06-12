package provider

type Token struct {
	Address  string
	Symbol   string
	Name     string
	Decimals uint32
	LogoURI  string
}

type Chain struct {
	ChainID                int64
	Name                   string
	Enabled                bool
	RequiredConfirmations  uint64
	DexTokenApproveAddress string
}

type QuoteRequest struct {
	ChainID          int64
	FromTokenAddress string
	ToTokenAddress   string
	Amount           string // minimal unit (wei), decimal string
	IncludeProtocols bool
}

type QuoteResponse struct {
	ChainID      int64
	Provider     string
	FromToken    Token
	ToToken      Token
	FromAmount   string
	ToAmount     string
	EstimatedGas uint64
	RawProtocols any    // best-effort, provider-specific (optional)
	TradeFee     string // DEX 交易手续费（USD 计价）
	PriceImpact  string // 价格冲击百分比（例如 "0.5" 表示 0.5%）
}

type UnsignedTransaction struct {
	ChainID         int64
	From            string
	To              string
	Data            string
	Value           string
	Gas             string
	GasPrice        string
	SlippagePercent string
	SignatureData   []string // 额外的签名数据（OKX特定：某些交易需要额外的签名数据）
}

type SwapRequest struct {
	ChainID          int64
	WalletAddress    string
	Recipient        string
	FromTokenAddress string
	ToTokenAddress   string
	Amount           string // minimal unit (wei), decimal string
	SlippageBps      int32
	EstimatedGas     uint64 // 预估 gas（询价时返回，用于执行 swap 时设置 gasLimit）
}

type SwapBuildResult struct {
	Provider string
	Quote    *QuoteResponse
	SwapTx   UnsignedTransaction
}

// ============================================================================
// OKX DEX Specific Types
// ============================================================================

// LiquiditySource represents a DEX or liquidity provider
type LiquiditySource struct {
	ID   string // DEX ID (e.g., "34" for Uniswap V2)
	Name string // DEX name (e.g., "Uniswap V2")
	Logo string // DEX logo URL
}

// ChainSwapStatus represents the status of a swap transaction on chain
type ChainSwapStatus int

const (
	ChainSwapStatusUnspecified ChainSwapStatus = iota
	ChainSwapStatusPending
	ChainSwapStatusSuccess
	ChainSwapStatusFailed
)

// TokenDetail represents token details in swap history
type TokenDetail struct {
	Symbol       string
	Amount       string // Token amount (in smallest unit, e.g., wei)
	TokenAddress string
}

// ChainSwapHistoryItem represents a swap transaction from OKX history API
type ChainSwapHistoryItem struct {
	ChainIndex     string          // Chain ID (e.g., "1" for Ethereum)
	TxHash         string          // Transaction hash
	BlockHeight    string          // Block number
	TxTime         int64           // Transaction time (Unix timestamp in milliseconds)
	Status         ChainSwapStatus // Transaction status
	TxType         string          // Transaction type (Approve, Wrap, Unwrap, Swap)
	FromAddress    string          // Sender address
	DexRouter      string          // DEX router address
	ToAddress      string          // Recipient address
	FromTokens     []TokenDetail   // Source token details
	ToTokens       []TokenDetail   // Destination token details
	ReferralAmount string          // Referral amount
	ErrorMsg       string          // Error message (if failed)
	GasLimit       string          // Gas limit
	GasUsed        string          // Gas used
	GasPrice       string          // Gas price
	TxFee          string          // Transaction fee
}
