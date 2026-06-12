package provider

import (
	"context"
	"time"

	"internalwallet/proto/pb"
)

type skipReceiptsCtxKey struct{}

// WithSkipReceipts marks ctx to skip receipt fetching in heavy block scans.
// Providers should honor this when fetching block transactions.
func WithSkipReceipts(ctx context.Context) context.Context {
	return context.WithValue(ctx, skipReceiptsCtxKey{}, true)
}

func ShouldSkipReceipts(ctx context.Context) bool {
	_, ok := ctx.Value(skipReceiptsCtxKey{}).(bool)
	return ok
}

// ProviderStatus describes current provider health.
type ProviderStatus int

const (
	ProviderStatusUnknown ProviderStatus = iota
	ProviderStatusHealthy
	ProviderStatusDegraded
	ProviderStatusUnhealthy
	ProviderStatusDisabled
)

func (s ProviderStatus) String() string {
	switch s {
	case ProviderStatusHealthy:
		return "healthy"
	case ProviderStatusDegraded:
		return "degraded"
	case ProviderStatusUnhealthy:
		return "unhealthy"
	case ProviderStatusDisabled:
		return "disabled"
	default:
		return "unknown"
	}
}

// Provider is the unified blockchain provider interface used by chainsync.
type Provider interface {
	// Metadata
	GetID() string
	GetType() pb.ProviderType
	GetChain() pb.BlockChainType
	GetEndpoint() string

	// Health & scoring
	IsHealthy() bool
	GetWeight() float64
	GetResponseTime() time.Duration
	GetSuccessRate() float64

	// Core API
	GetLatestBlock(ctx context.Context) (uint64, error)
	GetBlock(ctx context.Context, blockNumber uint64) (*pb.ChainBlockInfo, error)
	GetBlockTransactions(ctx context.Context, blockNumber uint64) ([]*pb.Transaction, error)
	GetTransaction(ctx context.Context, txHash string) (*pb.Transaction, error)
	GetAddressBalance(ctx context.Context, address string, tokens []string) (*pb.GetAddressBalanceResp, error)
	GetAddressTransactions(ctx context.Context, address string, startBlock, endBlock uint64, page, pageSize int32) (*pb.GetAddressTransactionsResp, error)

	// HealthCheck should be lightweight and bounded by ctx timeout.
	HealthCheck(ctx context.Context) error
}
