package okx

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"internalwallet/services/swap/rpc/internal/config"
	"internalwallet/services/swap/rpc/internal/provider"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Provider struct {
	client *Client

	chains map[int64]config.ChainConfig
}

func NewProvider(cfg config.OkxConfig, chains []config.ChainConfig) *Provider {
	m := make(map[int64]config.ChainConfig, len(chains))
	for _, c := range chains {
		if c.ChainID == 0 {
			continue
		}
		m[c.ChainID] = c
	}
	return &Provider{
		client: NewClient(cfg),
		chains: m,
	}
}

func (p *Provider) GetQuote(ctx context.Context, req *provider.QuoteRequest) (*provider.QuoteResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "nil request")
	}
	if !p.isChainEnabled(req.ChainID) {
		return nil, status.Error(codes.InvalidArgument, "unsupported chain_id")
	}

	// Build optional parameters
	var options *QuoteRequestOptions
	// TODO: 可选参数可从 provider.QuoteRequest 中扩展获取
	dto, err := p.client.GetQuote(ctx, req.ChainID, req.FromTokenAddress, req.ToTokenAddress, req.Amount, options)
	if err != nil {
		return nil, mapUpstreamError(err)
	}

	out, err := AdaptQuote(req.ChainID, req.FromTokenAddress, req.ToTokenAddress, dto)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to adapt quote")
	}
	return out, nil
}

func (p *Provider) BuildSwapTransaction(ctx context.Context, req *provider.SwapRequest) (*provider.SwapBuildResult, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "nil request")
	}
	if !p.isChainEnabled(req.ChainID) {
		return nil, status.Error(codes.InvalidArgument, "unsupported chain_id")
	}
	wallet := strings.TrimSpace(req.WalletAddress)
	if wallet == "" {
		return nil, status.Error(codes.InvalidArgument, "wallet_address is required")
	}

	// Build optional parameters
	var options *SwapRequestOptions
	// 如果前端传入了预估 gas，则在预估 gas 基础上上浮 20% 作为 gasLimit
	if req.EstimatedGas > 0 {
		// 预估 gas 上浮 20%
		gasLimit := req.EstimatedGas * 120 / 100
		options = &SwapRequestOptions{
			GasLimit: fmt.Sprintf("%d", gasLimit),
		}
	}
	dto, err := p.client.GetSwap(context.Background(), req.ChainID, req.FromTokenAddress, req.ToTokenAddress, req.Amount, wallet, strings.TrimSpace(req.Recipient), req.SlippageBps, options)
	if err != nil {
		return nil, mapUpstreamError(err)
	}

	out, err := AdaptSwap(req.ChainID, dto)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to adapt swap")
	}
	return out, nil
}

func (p *Provider) GetAllowance(ctx context.Context, chainID int64, tokenAddress, walletAddress string) (string, error) {
	// OKX DEX API does not provide an allowance query endpoint.
	// Return "0" to let callers generate approve_tx when requested.
	return "0", nil
}

func (p *Provider) GetApprovalTransaction(ctx context.Context, chainID int64, tokenAddress, amount, walletAddress string) (*provider.UnsignedTransaction, error) {
	if !p.isChainEnabled(chainID) {
		return nil, status.Error(codes.InvalidArgument, "unsupported chain_id")
	}

	tokenAddress = strings.TrimSpace(tokenAddress)
	if tokenAddress == "" {
		return nil, status.Error(codes.InvalidArgument, "token_address is required")
	}
	walletAddress = strings.TrimSpace(walletAddress)
	if walletAddress == "" {
		return nil, status.Error(codes.InvalidArgument, "wallet_address is required")
	}

	dto, err := p.client.GetApproveTransaction(ctx, chainID, tokenAddress, strings.TrimSpace(amount))
	if err != nil {
		return nil, mapUpstreamError(err)
	}

	out, err := AdaptApproveTx(chainID, walletAddress, tokenAddress, dto)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to adapt approve tx")
	}
	return out, nil
}

func (p *Provider) GetSupportedTokens(ctx context.Context, chainID int64) ([]provider.Token, error) {
	if !p.isChainEnabled(chainID) {
		return nil, status.Error(codes.InvalidArgument, "unsupported chain_id")
	}
	dto, err := p.client.GetAllTokens(ctx, chainID)
	if err != nil {
		return nil, mapUpstreamError(err)
	}
	return AdaptTokens(dto)
}

func (p *Provider) GetSupportedChains(ctx context.Context) ([]provider.Chain, error) {
	out := make([]provider.Chain, 0, len(p.chains))
	//for _, c := range p.chains {
	//	out = append(out, provider.Chain{
	//		ChainID:               c.ChainID,
	//		Name:                  strings.TrimSpace(c.Name),
	//		Enabled:               c.Enabled,
	//		RequiredConfirmations: c.RequiredConfirmations,
	//	})
	//}

	chains, _ := p.client.GetSupportedChains(context.Background())
	for _, c := range chains {
		out = append(out, provider.Chain{
			ChainID:                c.ChainIndex,
			Name:                   c.ChainName,
			Enabled:                true,
			DexTokenApproveAddress: c.DexTokenApproveAddress,
		})
	}
	return out, nil
}

// ============================================================================
// ExtendedProvider Implementation
// ============================================================================

// GetLiquiditySources returns available DEX/liquidity sources from OKX for a given chain
func (p *Provider) GetLiquiditySources(ctx context.Context, chainID int64) ([]provider.LiquiditySource, error) {
	if !p.isChainEnabled(chainID) {
		return nil, status.Error(codes.InvalidArgument, "unsupported chain_id")
	}
	dtos, err := p.client.GetLiquiditySources(ctx, chainID)
	if err != nil {
		return nil, mapUpstreamError(err)
	}
	return AdaptLiquiditySources(dtos), nil
}

// GetExternalSwapStatus queries the status of a swap transaction by tx hash from OKX
func (p *Provider) GetExternalSwapStatus(ctx context.Context, chainID int64, txHash string, isFromMyProject bool) (*provider.ChainSwapHistoryItem, error) {
	if chainID <= 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid chain_id")
	}
	txHash = strings.TrimSpace(txHash)
	if txHash == "" {
		return nil, status.Error(codes.InvalidArgument, "tx_hash is required")
	}

	dto, err := p.client.GetSwapHistory(ctx, chainID, txHash, isFromMyProject)
	if err != nil {
		return nil, mapUpstreamError(err)
	}

	// 检查返回的数据是否为空（OKX API 可能返回空数据但无错误）
	if dto == nil || (strings.TrimSpace(dto.TxHash) == "" && strings.TrimSpace(dto.ChainIndex) == "") {
		// 返回一个空的结果对象，而不是错误（因为交易可能还未被 OKX 索引）
		return &provider.ChainSwapHistoryItem{
			Status: provider.ChainSwapStatusUnspecified,
		}, nil
	}

	return AdaptSwapHistory(dto)
}

func (p *Provider) isChainEnabled(chainID int64) bool {
	c, ok := p.chains[chainID]
	return ok && c.Enabled
}

func mapUpstreamError(err error) error {
	if err == nil {
		return nil
	}

	var apiErr *apiError
	if errors.As(err, &apiErr) && apiErr != nil {
		switch apiErr.StatusCode {
		case 400:
			return status.Error(codes.InvalidArgument, "upstream bad request")
		case 401, 403:
			return status.Error(codes.PermissionDenied, "upstream unauthorized")
		case 404:
			return status.Error(codes.NotFound, "upstream not found")
		case 429:
			return status.Error(codes.ResourceExhausted, "upstream rate limited")
		default:
			if apiErr.StatusCode >= 500 && apiErr.StatusCode <= 599 {
				return status.Error(codes.Unavailable, "upstream unavailable")
			}
		}

		// OKX DEX API may return HTTP 200 with code != 0 for parameter issues.
		if apiErr.StatusCode >= 200 && apiErr.StatusCode < 300 && strings.TrimSpace(apiErr.Code) != "" && apiErr.Code != "0" {
			return status.Error(codes.InvalidArgument, fmt.Sprintf("upstream error: %s", apiErr.Msg))
		}

		return status.Error(codes.Internal, "upstream error")
	}

	return status.Error(codes.Unavailable, fmt.Sprintf("upstream request failed: %v", err))
}
