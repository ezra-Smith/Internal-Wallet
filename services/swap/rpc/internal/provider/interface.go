package provider

import "context"

type SwapProvider interface {
	GetQuote(ctx context.Context, req *QuoteRequest) (*QuoteResponse, error)
	BuildSwapTransaction(ctx context.Context, req *SwapRequest) (*SwapBuildResult, error)

	// Allowance / approve tx helpers (best-effort; provider-specific)
	GetAllowance(ctx context.Context, chainID int64, tokenAddress, walletAddress string) (string, error)
	GetApprovalTransaction(ctx context.Context, chainID int64, tokenAddress, amount, walletAddress string) (*UnsignedTransaction, error)

	GetSupportedTokens(ctx context.Context, chainID int64) ([]Token, error)
	GetSupportedChains(ctx context.Context) ([]Chain, error)
}

// ExtendedProvider defines optional provider-specific methods
// Not all providers implement all methods - check provider documentation
type ExtendedProvider interface {
	SwapProvider

	// GetLiquiditySources returns available DEX/liquidity sources for a given chain
	// OKX-specific: returns list of supported DEXs (Uniswap V2, SushiSwap, etc.)
	GetLiquiditySources(ctx context.Context, chainID int64) ([]LiquiditySource, error)

	// GetExternalSwapStatus queries the status of a swap transaction by tx hash
	// OKX-specific: queries OKX's /history endpoint
	GetExternalSwapStatus(ctx context.Context, chainID int64, txHash string, isFromMyProject bool) (*ChainSwapHistoryItem, error)
}
