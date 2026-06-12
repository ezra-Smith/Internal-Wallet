package provider

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"internalwallet/proto/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

// Pool manages all chain providers with health checks, scoring and failover.
type Pool struct {
	providers map[pb.BlockChainType][]*ManagedProvider
	mu        sync.RWMutex

	config *PoolConfig

	healthCheckTicker *time.Ticker
	stopCh            chan struct{}

	startOnce sync.Once
	stopOnce  sync.Once
}

type PoolConfig struct {
	HealthCheckInterval   time.Duration
	FailoverThreshold     int
	RecoveryCheckInterval time.Duration
	MaxConcurrentRequests int
}

func DefaultPoolConfig() *PoolConfig {
	return &PoolConfig{
		HealthCheckInterval:   30 * time.Second,
		FailoverThreshold:     3,
		RecoveryCheckInterval: 5 * time.Minute,
		MaxConcurrentRequests: 200,
	}
}

func NewPool(config *PoolConfig) *Pool {
	if config == nil {
		config = DefaultPoolConfig()
	}

	return &Pool{
		providers: make(map[pb.BlockChainType][]*ManagedProvider),
		config:    config,
		stopCh:    make(chan struct{}),
	}
}

type ManagedProvider struct {
	Provider       Provider
	LastError      error
	LastSuccess    time.Time
	LastFailure    time.Time
	FailureCount   int
	TotalRequests  int64
	FailedRequests int64
	Status         ProviderStatus
	mu             sync.RWMutex
}

func (p *Pool) AddProvider(provider Provider) {
	p.mu.Lock()
	defer p.mu.Unlock()

	chain := provider.GetChain()
	managed := &ManagedProvider{
		Provider:    provider,
		Status:      ProviderStatusHealthy,
		LastSuccess: time.Now(),
	}

	p.providers[chain] = append(p.providers[chain], managed)
	logx.Infof("✅ Added provider %s for chain %s", provider.GetID(), chain.String())
}

func (p *Pool) GetProvider(chain pb.BlockChainType) (Provider, error) {
	p.mu.RLock()
	providers := p.providers[chain]
	p.mu.RUnlock()

	if len(providers) == 0 {
		return nil, fmt.Errorf("no providers available for chain %s", chain.String())
	}

	calculateScore := func(provider Provider) float64 {
		score := provider.GetSuccessRate()
		if weight := provider.GetWeight(); weight > 0 {
			score *= weight
		}
		if rt := provider.GetResponseTime(); rt > 0 {
			score = score / (1 + rt.Seconds())
		}
		return score
	}

	var best *ManagedProvider
	bestScore := float64(-1)

	for _, mp := range providers {
		mp.mu.RLock()
		status := mp.Status
		mp.mu.RUnlock()

		if status == ProviderStatusDisabled || status == ProviderStatusUnhealthy {
			continue
		}
		if !mp.Provider.IsHealthy() {
			continue
		}

		score := calculateScore(mp.Provider)
		if score > bestScore {
			bestScore = score
			best = mp
		}
	}

	if best == nil {
		for _, mp := range providers {
			mp.mu.RLock()
			if mp.Status != ProviderStatusDisabled {
				best = mp
				mp.mu.RUnlock()
				break
			}
			mp.mu.RUnlock()
		}
	}

	if best == nil {
		return nil, fmt.Errorf("no available providers for chain %s", chain.String())
	}

	return best.Provider, nil
}

func (p *Pool) GetAllProviders(chain pb.BlockChainType) []Provider {
	p.mu.RLock()
	defer p.mu.RUnlock()

	managedProviders := p.providers[chain]
	providers := make([]Provider, 0, len(managedProviders))
	for _, mp := range managedProviders {
		providers = append(providers, mp.Provider)
	}
	return providers
}

func (p *Pool) RecordSuccess(provider Provider) {
	p.mu.RLock()
	providers := p.providers[provider.GetChain()]
	p.mu.RUnlock()

	for _, mp := range providers {
		if mp.Provider.GetID() == provider.GetID() {
			mp.mu.Lock()
			mp.LastSuccess = time.Now()
			mp.TotalRequests++
			mp.FailureCount = 0
			if mp.Status == ProviderStatusDegraded {
				mp.Status = ProviderStatusHealthy
			}
			mp.mu.Unlock()
			break
		}
	}
}

func (p *Pool) RecordFailure(provider Provider, err error) {
	p.mu.RLock()
	providers := p.providers[provider.GetChain()]
	p.mu.RUnlock()

	for _, mp := range providers {
		if mp.Provider.GetID() == provider.GetID() {
			mp.mu.Lock()
			mp.LastFailure = time.Now()
			mp.LastError = err
			mp.TotalRequests++
			mp.FailedRequests++
			mp.FailureCount++

			if mp.FailureCount >= p.config.FailoverThreshold {
				mp.Status = ProviderStatusUnhealthy
				logx.Errorf("❌ Provider %s marked as unhealthy after %d failures", provider.GetID(), mp.FailureCount)
			} else if mp.FailureCount >= p.config.FailoverThreshold/2 {
				mp.Status = ProviderStatusDegraded
			}
			mp.mu.Unlock()
			break
		}
	}
}

func (p *Pool) StartHealthCheck() {
	p.startOnce.Do(func() {
		p.healthCheckTicker = time.NewTicker(p.config.HealthCheckInterval)
		go p.runHealthCheck()
		logx.Info("🏥 Health check started for provider pool")
	})
}

func (p *Pool) runHealthCheck() {
	for {
		select {
		case <-p.healthCheckTicker.C:
			p.checkAllProviders()
		case <-p.stopCh:
			return
		}
	}
}

func (p *Pool) checkAllProviders() {
	p.mu.RLock()
	all := make([]*ManagedProvider, 0)
	for _, providers := range p.providers {
		all = append(all, providers...)
	}
	p.mu.RUnlock()

	if len(all) == 0 {
		return
	}

	maxConcurrent := 4
	if len(all) < maxConcurrent {
		maxConcurrent = len(all)
	}

	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	for _, mp := range all {
		wg.Add(1)
		sem <- struct{}{}
		go func(mp *ManagedProvider) {
			defer wg.Done()
			defer func() { <-sem }()
			p.checkProvider(mp)
		}(mp)
	}
	wg.Wait()
}

func (p *Pool) checkProvider(mp *ManagedProvider) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := mp.Provider.HealthCheck(ctx)

	mp.mu.Lock()
	defer mp.mu.Unlock()

	if err != nil {
		mp.LastError = err
		mp.LastFailure = time.Now()
		mp.FailureCount++
		if mp.FailureCount >= p.config.FailoverThreshold {
			mp.Status = ProviderStatusUnhealthy
		}
		return
	}

	mp.LastSuccess = time.Now()
	mp.FailureCount = 0
	mp.Status = ProviderStatusHealthy
}

func (p *Pool) GetStats() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := make(map[string]interface{})
	chainStats := make(map[string][]map[string]interface{})

	for chain, providers := range p.providers {
		providerStats := make([]map[string]interface{}, 0)
		for _, mp := range providers {
			mp.mu.RLock()
			providerStats = append(providerStats, map[string]interface{}{
				"id":              mp.Provider.GetID(),
				"status":          mp.Status.String(),
				"success_rate":    mp.Provider.GetSuccessRate(),
				"total_requests":  mp.TotalRequests,
				"failed_requests": mp.FailedRequests,
				"failure_count":   mp.FailureCount,
				"last_success":    mp.LastSuccess,
				"last_failure":    mp.LastFailure,
			})
			mp.mu.RUnlock()
		}
		chainStats[chain.String()] = providerStats
	}

	stats["providers"] = chainStats
	return stats
}

func (p *Pool) GetHealthyProviderCount(chain pb.BlockChainType) int {
	p.mu.RLock()
	providers := p.providers[chain]
	p.mu.RUnlock()

	count := 0
	for _, mp := range providers {
		mp.mu.RLock()
		if mp.Status == ProviderStatusHealthy {
			count++
		}
		mp.mu.RUnlock()
	}
	return count
}

func (p *Pool) GetProvidersByPriority(chain pb.BlockChainType) []Provider {
	p.mu.RLock()
	managedProviders := p.providers[chain]
	p.mu.RUnlock()

	if len(managedProviders) == 0 {
		return nil
	}

	sorted := make([]*ManagedProvider, len(managedProviders))
	copy(sorted, managedProviders)

	sort.Slice(sorted, func(i, j int) bool {
		sorted[i].mu.RLock()
		sorted[j].mu.RLock()
		defer sorted[i].mu.RUnlock()
		defer sorted[j].mu.RUnlock()

		if sorted[i].Status != sorted[j].Status {
			return sorted[i].Status < sorted[j].Status
		}
		return sorted[i].Provider.GetSuccessRate() > sorted[j].Provider.GetSuccessRate()
	})

	result := make([]Provider, len(sorted))
	for i, mp := range sorted {
		result[i] = mp.Provider
	}
	return result
}

func (p *Pool) Stop() {
	p.stopOnce.Do(func() {
		if p.healthCheckTicker != nil {
			p.healthCheckTicker.Stop()
		}
		close(p.stopCh)
		logx.Info("Provider pool stopped")
	})
}
