package evm

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
	"internalwallet/services/chainsync/rpc/internal/provider"
)

var _ provider.Provider = (*Web3Provider)(nil)

// ClientPool is a minimal ethclient pool (mainly for batched reads).
type ClientPool struct {
	clients     []*ethclient.Client
	available   chan *ethclient.Client
	mu          sync.Mutex
	maxSize     int
	currentSize int
}

// Web3Provider is a go-ethereum backed provider (ETH/BSC etc).
type Web3Provider struct {
	id       string
	chain    pb.BlockChainType
	endpoint string
	client   *ethclient.Client // main client for non-batch ops
	pool     *ClientPool       // pool for batch ops
	chainID  *big.Int
	weight   float64

	mu             sync.RWMutex
	isHealthy      bool
	lastCheck      time.Time
	responseTime   time.Duration
	totalRequests  int64
	failedRequests int64
}

func NewWeb3Provider(id string, chain pb.BlockChainType, endpoint string, chainID int64, weight float64) (*Web3Provider, error) {
	logx.Infof("🔌 Connecting to blockchain endpoint: %s (ChainID: %d)", endpoint, chainID)

	var client *ethclient.Client
	var err error

	maxRetries := 3
	for retry := 0; retry < maxRetries; retry++ {
		logx.Infof("🔄 Attempt %d/%d to connect to %s", retry+1, maxRetries, endpoint)

		client, err = ethclient.Dial(endpoint)
		if err == nil {
			logx.Infof("✅ Connected successfully")
			break
		}

		logx.Errorf("❌ Connection attempt %d failed: %v", retry+1, err)
		if retry < maxRetries-1 {
			time.Sleep(time.Duration(retry+1) * time.Second)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect to ethereum client after %d attempts: %v", maxRetries, err)
	}

	pool := &ClientPool{
		clients:     []*ethclient.Client{client},     // reuse main connection
		available:   make(chan *ethclient.Client, 1), // pool size 1
		maxSize:     1,
		currentSize: 1,
	}
	pool.available <- client

	if weight <= 0 {
		weight = 1.0
	}

	return &Web3Provider{
		id:        id,
		chain:     chain,
		endpoint:  endpoint,
		client:    client,
		pool:      pool,
		chainID:   big.NewInt(chainID),
		weight:    weight,
		isHealthy: true,
	}, nil
}

func (p *Web3Provider) GetID() string { return p.id }

func (p *Web3Provider) GetType() pb.ProviderType {
	return pb.ProviderType_PROVIDER_TYPE_QUICKNODE
}

func (p *Web3Provider) GetChain() pb.BlockChainType { return p.chain }

func (p *Web3Provider) GetEndpoint() string { return p.endpoint }

func (p *Web3Provider) IsHealthy() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.isHealthy
}

func (p *Web3Provider) GetWeight() float64 { return p.weight }

func (p *Web3Provider) GetResponseTime() time.Duration {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.responseTime
}

func (p *Web3Provider) GetSuccessRate() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.totalRequests == 0 {
		return 100.0
	}
	return float64(p.totalRequests-p.failedRequests) * 100.0 / float64(p.totalRequests)
}

func (p *Web3Provider) recordRequest(success bool, responseTime time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.totalRequests++
	p.responseTime = responseTime

	if success {
		p.isHealthy = true
	} else {
		p.failedRequests++
		p.isHealthy = false
	}

	p.lastCheck = time.Now()
}

func (p *Web3Provider) finishRequest(start time.Time, err *error) {
	if r := recover(); r != nil {
		*err = fmt.Errorf("panic: %v", r)
	}
	p.recordRequest(*err == nil, time.Since(start))
}

func (p *Web3Provider) HealthCheck(ctx context.Context) error {
	_, err := p.client.BlockNumber(ctx)
	if err != nil {
		p.recordRequest(false, 0)
		return fmt.Errorf("health check failed: %v", err)
	}
	p.recordRequest(true, 0)
	return nil
}

func (p *Web3Provider) GetClientFromPool(ctx context.Context) (*ethclient.Client, error) {
	if p.pool == nil {
		return nil, fmt.Errorf("connection pool not initialized")
	}

	poolCtx, poolCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer poolCancel()

	select {
	case client := <-p.pool.available:
		return client, nil
	case <-poolCtx.Done():
		return nil, fmt.Errorf("timeout waiting for available client from pool")
	case <-ctx.Done():
		return nil, fmt.Errorf("request canceled")
	}
}

func (p *Web3Provider) ReturnClientToPool(client *ethclient.Client) {
	if p.pool == nil || client == nil {
		return
	}

	select {
	case p.pool.available <- client:
	default:
		client.Close()
	}
}

func (p *Web3Provider) Close() {
	if p.client != nil {
		p.client.Close()
	}

	if p.pool != nil {
		for _, client := range p.pool.clients {
			if client != nil {
				client.Close()
			}
		}
		close(p.pool.available)
	}
}
