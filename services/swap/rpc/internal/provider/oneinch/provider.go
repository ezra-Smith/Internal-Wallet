package oneinch

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"internalwallet/services/swap/rpc/internal/config"
	"internalwallet/services/swap/rpc/internal/provider"
)

type Provider struct {
	client *Client

	chains map[int64]config.ChainConfig
}

func NewProvider(cfg config.OneInchConfig, chains []config.ChainConfig) *Provider {
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

	dto, err := p.client.GetQuote(ctx, req.ChainID, req.FromTokenAddress, req.ToTokenAddress, req.Amount, req.IncludeProtocols)
	if err != nil {
		return nil, mapUpstreamError(err)
	}
	out, err := AdaptQuote(req.ChainID, dto)
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

	receiver := strings.TrimSpace(req.Recipient)
	dto, err := p.client.GetSwap(ctx, req.ChainID, req.FromTokenAddress, req.ToTokenAddress, req.Amount, wallet, receiver, req.SlippageBps)
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
	if !p.isChainEnabled(chainID) {
		return "", status.Error(codes.InvalidArgument, "unsupported chain_id")
	}
	dto, err := p.client.GetAllowance(ctx, chainID, strings.TrimSpace(tokenAddress), strings.TrimSpace(walletAddress))
	if err != nil {
		return "", mapUpstreamError(err)
	}
	if dto == nil || strings.TrimSpace(dto.Allowance) == "" {
		return "0", nil
	}
	return strings.TrimSpace(dto.Allowance), nil
}

func (p *Provider) GetApprovalTransaction(ctx context.Context, chainID int64, tokenAddress, amount, walletAddress string) (*provider.UnsignedTransaction, error) {
	if !p.isChainEnabled(chainID) {
		return nil, status.Error(codes.InvalidArgument, "unsupported chain_id")
	}
	dto, err := p.client.GetApproveTransaction(ctx, chainID, strings.TrimSpace(tokenAddress), strings.TrimSpace(amount))
	if err != nil {
		return nil, mapUpstreamError(err)
	}
	out, err := AdaptApproveTx(chainID, strings.TrimSpace(walletAddress), dto)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to adapt approve tx")
	}
	return out, nil
}

func (p *Provider) GetSupportedTokens(ctx context.Context, chainID int64) ([]provider.Token, error) {
	if !p.isChainEnabled(chainID) {
		return nil, status.Error(codes.InvalidArgument, "unsupported chain_id")
	}
	dto, err := p.client.GetTokens(ctx, chainID)
	if err != nil {
		return nil, mapUpstreamError(err)
	}
	return AdaptTokens(dto)
}

func (p *Provider) GetSupportedChains(ctx context.Context) ([]provider.Chain, error) {
	out := make([]provider.Chain, 0, len(p.chains))
	for _, c := range p.chains {
		out = append(out, provider.Chain{
			ChainID:               c.ChainID,
			Name:                  strings.TrimSpace(c.Name),
			Enabled:               c.Enabled,
			RequiredConfirmations: c.RequiredConfirmations,
		})
	}
	return out, nil
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
		return status.Error(codes.Internal, "upstream error")
	}
	return status.Error(codes.Unavailable, fmt.Sprintf("upstream request failed: %v", err))
}
