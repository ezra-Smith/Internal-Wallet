package svc

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
	"internalwallet/services/chainsync/rpc/internal/config"
	"internalwallet/services/chainsync/rpc/internal/provider"
)

// HeadTracker keeps per-chain latest block heights (heads) fresh and persisted.
//
// It is intentionally small:
// - reads are in-memory (fast, no Redis on hot path)
// - background polling refreshes heads from providers
// - successful refreshes persist to Redis for startup warm-cache
type HeadTracker struct {
	pool  *provider.Pool
	redis *RedisCacheManager
	cfg   *config.Config

	mu        sync.RWMutex
	heads     map[pb.BlockChainType]uint64
	headTimes map[pb.BlockChainType]time.Time

	refreshInterval time.Duration
	requestTimeout  time.Duration

	startOnce sync.Once
	stopOnce  sync.Once
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

func NewHeadTracker(pool *provider.Pool, redis *RedisCacheManager, cfg *config.Config) *HeadTracker {
	interval := 5 * time.Second
	if cfg != nil && cfg.Monitoring.AddressMonitoring != nil && cfg.Monitoring.AddressMonitoring.CheckInterval > 0 {
		interval = time.Duration(cfg.Monitoring.AddressMonitoring.CheckInterval) * time.Second
	}
	if interval < 2*time.Second {
		interval = 2 * time.Second
	}
	if interval > 30*time.Second {
		interval = 30 * time.Second
	}

	return &HeadTracker{
		pool:            pool,
		redis:           redis,
		cfg:             cfg,
		heads:           make(map[pb.BlockChainType]uint64, 3),
		headTimes:       make(map[pb.BlockChainType]time.Time, 3),
		refreshInterval: interval,
		requestTimeout:  10 * time.Second,
	}
}

func (ht *HeadTracker) Start(parent context.Context) {
	if ht == nil {
		return
	}
	if parent == nil {
		parent = context.Background()
	}
	ht.startOnce.Do(func() {
		ctx, cancel := context.WithCancel(parent)
		ht.cancel = cancel

		for _, chain := range enabledChains(ht.cfg) {
			chain := chain
			ht.wg.Add(1)
			go func() {
				defer ht.wg.Done()
				ht.pollChain(ctx, chain)
			}()
		}
		logx.Infof("✅ HeadTracker started (interval=%s)", ht.refreshInterval)
	})
}

func (ht *HeadTracker) Stop() {
	if ht == nil {
		return
	}
	ht.stopOnce.Do(func() {
		if ht.cancel != nil {
			ht.cancel()
		}
		ht.wg.Wait()
		logx.Info("✅ HeadTracker stopped")
	})
}

func (ht *HeadTracker) Get(chain pb.BlockChainType) (uint64, bool) {
	if ht == nil {
		return 0, false
	}
	ht.mu.RLock()
	defer ht.mu.RUnlock()
	v, ok := ht.heads[chain]
	return v, ok && v > 0
}

func (ht *HeadTracker) Set(chain pb.BlockChainType, head uint64) {
	if ht == nil || head == 0 {
		return
	}
	ht.mu.Lock()
	cur := ht.heads[chain]
	if head > cur {
		ht.heads[chain] = head
		ht.headTimes[chain] = time.Now()
	}
	ht.mu.Unlock()
}

func enabledChains(cfg *config.Config) []pb.BlockChainType {
	// Default to all supported chains if config is missing to keep dev flows working.
	if cfg == nil {
		return []pb.BlockChainType{
			pb.BlockChainType_CHAIN_TYPE_ETHEREUM,
			pb.BlockChainType_CHAIN_TYPE_BSC,
			pb.BlockChainType_CHAIN_TYPE_TRON,
		}
	}

	out := make([]pb.BlockChainType, 0, 3)
	if cfg.Chains.Ethereum.Enabled {
		out = append(out, pb.BlockChainType_CHAIN_TYPE_ETHEREUM)
	}
	if cfg.Chains.BSC.Enabled {
		out = append(out, pb.BlockChainType_CHAIN_TYPE_BSC)
	}
	if cfg.Chains.Tron.Enabled {
		out = append(out, pb.BlockChainType_CHAIN_TYPE_TRON)
	}
	if len(out) == 0 {
		// If all are disabled, still track none.
		return nil
	}
	return out
}

func (ht *HeadTracker) pollChain(ctx context.Context, chain pb.BlockChainType) {
	// Warm from Redis (best-effort).
	if ht.redis != nil {
		if cached, exists, err := ht.redis.GetLatestBlockFromRedis(chain); err == nil && exists {
			ht.Set(chain, cached)
		}
	}

	// Refresh immediately on start.
	_ = ht.refreshOnce(ctx, chain)

	ticker := time.NewTicker(ht.refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = ht.refreshOnce(ctx, chain)
		}
	}
}

func (ht *HeadTracker) refreshOnce(parent context.Context, chain pb.BlockChainType) error {
	if ht.pool == nil {
		return fmt.Errorf("provider pool is nil")
	}

	p, err := ht.pool.GetProvider(chain)
	if err != nil {
		return fmt.Errorf("get provider for %v: %w", chain, err)
	}

	ctx, cancel := context.WithTimeout(parent, ht.requestTimeout)
	defer cancel()

	head, err := p.GetLatestBlock(ctx)
	if err != nil {
		return fmt.Errorf("get latest block for %v via %s: %w", chain, p.GetID(), err)
	}

	ht.Set(chain, head)

	// Persist (no expiration). Startup will still refresh on its own schedule.
	if ht.redis != nil {
		if setErr := ht.redis.SetLatestBlockToRedis(chain, head, 0); setErr != nil {
			logx.Errorf("HeadTracker: persist head failed: chain=%v head=%d err=%v", chain, head, setErr)
		}
	}

	return nil
}

