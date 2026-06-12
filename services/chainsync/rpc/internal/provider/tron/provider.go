package tron

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/fbsobreira/gotron-sdk/pkg/client"
	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
	"internalwallet/services/chainsync/rpc/internal/provider"
)

var _ provider.Provider = (*TronProvider)(nil)

// TronProvider implements provider.Provider for TRON using gotron-sdk.
type TronProvider struct {
	id       string
	chain    pb.BlockChainType
	endpoint string
	apiKey   string
	chainID  *big.Int
	weight   float64

	mu             sync.RWMutex
	totalRequests  int64
	failedRequests int64
	lastRespTime   time.Duration
	isHealthy      bool
	lastCheck      time.Time
}

func NewTronProvider(id string, chain pb.BlockChainType, endpoint string, chainID int64, apiKey string, weight float64) (*TronProvider, error) {
	if endpoint == "" {
		return nil, fmt.Errorf("empty endpoint")
	}

	if weight <= 0 {
		weight = 1.0
	}

	provider := &TronProvider{
		id:        id,
		chain:     chain,
		endpoint:  endpoint,
		apiKey:    apiKey,
		chainID:   big.NewInt(chainID),
		weight:    weight,
		isHealthy: true,
	}

	logx.Infof("✅ TronProvider %s initialized with per-request connection at %s (ChainID: %d)", id, endpoint, chainID)
	return provider, nil
}

func (t *TronProvider) createClient() *client.GrpcClient {
	grpcClient := client.NewGrpcClient(t.endpoint)
	if t.apiKey != "" {
		grpcClient.SetAPIKey(t.apiKey)
	}
	return grpcClient
}

func (t *TronProvider) finishRequest(start time.Time, err *error) {
	if r := recover(); r != nil {
		*err = fmt.Errorf("panic: %v", r)
	}
	t.recordRequest(*err == nil, time.Since(start))
}

func (t *TronProvider) connectWithTimeout(ctx context.Context, c *client.GrpcClient) error {
	done := make(chan error, 1)
	go func() {
		done <- c.Start(client.GRPCInsecure())
	}()

	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("连接失败: %v", err)
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("连接超时: %v", ctx.Err())
	}
}

func (t *TronProvider) GetID() string { return t.id }

func (t *TronProvider) GetType() pb.ProviderType {
	// Most TRON nodes are self-hosted in this project.
	return pb.ProviderType_PROVIDER_TYPE_SELF_HOSTED
}

func (t *TronProvider) GetChain() pb.BlockChainType { return t.chain }

func (t *TronProvider) GetEndpoint() string { return t.endpoint }

func (t *TronProvider) GetWeight() float64 { return t.weight }

func (t *TronProvider) recordRequest(success bool, dur time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.totalRequests++
	t.lastRespTime = dur
	if !success {
		t.failedRequests++
		t.isHealthy = false
	} else {
		t.isHealthy = true
	}
	t.lastCheck = time.Now()
}

func (t *TronProvider) IsHealthy() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.isHealthy
}

func (t *TronProvider) GetResponseTime() time.Duration {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.lastRespTime
}

func (t *TronProvider) GetSuccessRate() float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.totalRequests == 0 {
		return 100.0
	}
	return float64(t.totalRequests-t.failedRequests) * 100.0 / float64(t.totalRequests)
}
