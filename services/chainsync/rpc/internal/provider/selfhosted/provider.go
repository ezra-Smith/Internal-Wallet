package selfhosted

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
	"internalwallet/services/chainsync/rpc/internal/provider"
)

var _ provider.Provider = (*SelfHostedProvider)(nil)

type SelfHostedProvider struct {
	id       string
	config   *Config
	client   *http.Client
	chainID  int64
	endpoint string
	weight   float64

	mu             sync.RWMutex
	isHealthy      bool
	lastCheck      time.Time
	responseTime   time.Duration
	totalRequests  int64
	failedRequests int64
}

func NewSelfHostedProvider(config *Config) (*SelfHostedProvider, error) {
	if config == nil {
		return nil, fmt.Errorf("nil config")
	}

	logx.Infof("🏠 Creating self-hosted RPC provider: %s (Type: %s)", config.Name, config.Type)

	endpoint := config.endpoint()
	if endpoint == "" {
		return nil, fmt.Errorf("invalid endpoint configuration")
	}

	client := &http.Client{
		Timeout: 60 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 5,
			IdleConnTimeout:     90 * time.Second,
			DisableCompression:  false,
			DialContext: (&net.Dialer{
				Timeout:   15 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ResponseHeaderTimeout: 30 * time.Second,
			TLSHandshakeTimeout:   15 * time.Second,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	baseURL := strings.TrimRight(config.Endpoint, "/")
	switch config.Type {
	case "ethereum":
		baseURL = strings.TrimSuffix(baseURL, "/eth")
	case "bsc":
		baseURL = strings.TrimSuffix(baseURL, "/bsc")
	case "tron":
		baseURL = strings.TrimSuffix(baseURL, "/tron")
	}

	if err := testConnection(ctx, client, baseURL, config.Chain); err != nil {
		return nil, fmt.Errorf("failed to test connection: %v", err)
	}

	logx.Infof("✅ Self-hosted RPC provider connected successfully: %s (%s)", config.Name, endpoint)

	weight := config.Weight
	if weight <= 0 {
		weight = 1.0
	}

	return &SelfHostedProvider{
		id:        fmt.Sprintf("self-hosted-%s", strings.ToLower(strings.ReplaceAll(config.Name, " ", "-"))),
		config:    config,
		client:    client,
		chainID:   config.ChainID,
		endpoint:  endpoint,
		weight:    weight,
		isHealthy: true,
	}, nil
}

func testConnection(ctx context.Context, client *http.Client, endpoint string, chain pb.BlockChainType) error {
	logx.Infof("🔍 Testing connection for chain type: %v (%d)", chain, int(chain))

	healthURL := strings.TrimRight(endpoint, "/") + "/health"
	logx.Infof("🔍 Testing health endpoint: %s", healthURL)

	req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %v", err)
	}

	testCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req = req.WithContext(testCtx)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("health check request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed with status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read health check response: %v", err)
	}

	var healthResp struct {
		Status    string `json:"status"`
		Timestamp string `json:"timestamp"`
	}
	if err := json.Unmarshal(body, &healthResp); err != nil {
		return fmt.Errorf("failed to decode health check response: %v", err)
	}

	if healthResp.Status != "ok" {
		return fmt.Errorf("health check returned non-ok status: %s", healthResp.Status)
	}

	logx.Infof("✅ Health check passed for endpoint %s (status: %s, timestamp: %s)", endpoint, healthResp.Status, healthResp.Timestamp)
	return nil
}

func (p *SelfHostedProvider) GetID() string { return p.id }

func (p *SelfHostedProvider) GetType() pb.ProviderType {
	return pb.ProviderType_PROVIDER_TYPE_SELF_HOSTED
}

func (p *SelfHostedProvider) GetChain() pb.BlockChainType { return p.config.Chain }

func (p *SelfHostedProvider) GetEndpoint() string { return p.endpoint }

func (p *SelfHostedProvider) IsHealthy() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.isHealthy
}

func (p *SelfHostedProvider) GetWeight() float64 { return p.weight }

func (p *SelfHostedProvider) GetResponseTime() time.Duration {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.responseTime
}

func (p *SelfHostedProvider) GetSuccessRate() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.totalRequests == 0 {
		return 100.0
	}
	return float64(p.totalRequests-p.failedRequests) * 100.0 / float64(p.totalRequests)
}

func (p *SelfHostedProvider) recordRequest(success bool, responseTime time.Duration) {
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

func (p *SelfHostedProvider) RecordRequest(success bool, responseTime time.Duration) {
	p.recordRequest(success, responseTime)
}

func (p *SelfHostedProvider) finishRequest(start time.Time, err *error) {
	if r := recover(); r != nil {
		*err = fmt.Errorf("panic: %v", r)
	}
	p.RecordRequest(*err == nil, time.Since(start))
}
