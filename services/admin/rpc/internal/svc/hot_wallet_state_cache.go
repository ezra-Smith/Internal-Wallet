package svc

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"internalwallet/proto/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

const DefaultHotWalletSeedID = "hot_wallet_main"

type hotWalletStateCache struct {
	mu        sync.Mutex
	state     string
	fetchedAt time.Time
	ttl       time.Duration
}

func newHotWalletStateCache(ttl time.Duration) *hotWalletStateCache {
	if ttl <= 0 {
		ttl = 2 * time.Second
	}
	return &hotWalletStateCache{ttl: ttl}
}

// ForceSetCachedHotWalletState updates cached wallet state immediately.
// It is best-effort and only affects the current admin-rpc process memory.
func (svcCtx *ServiceContext) ForceSetCachedHotWalletState(state string) {
	if svcCtx == nil || svcCtx.hotWalletStateCache == nil {
		return
	}
	state = strings.TrimSpace(state)
	if state == "" {
		return
	}

	cache := svcCtx.hotWalletStateCache
	now := time.Now()
	cache.mu.Lock()
	cache.state = state
	cache.fetchedAt = now
	cache.mu.Unlock()
}

// RefreshHotWalletStateCache queries signer runtime status and updates cache.
func (svcCtx *ServiceContext) RefreshHotWalletStateCache(ctx context.Context) (string, error) {
	if svcCtx == nil {
		return "", fmt.Errorf("service context is nil")
	}
	if svcCtx.SignerRpc == nil {
		return "", fmt.Errorf("signer rpc not available")
	}

	st, err := svcCtx.SignerRpc.GetRuntimeStatus(ctx, &pb.GetRuntimeStatusRequest{SeedId: DefaultHotWalletSeedID})
	if err != nil || st == nil {
		return "", fmt.Errorf("failed to query signer runtime status")
	}
	state := strings.TrimSpace(st.GetWalletState())
	if state != "" {
		svcCtx.ForceSetCachedHotWalletState(state)
	}
	return state, nil
}

// GetCachedHotWalletState returns signer hot wallet state with a short in-memory cache.
//
// Policy:
// - If Signer is unavailable or returns error, we return a non-nil error.
// - Callers that enforce onboarding should treat error as "not unlocked" (conservative).
func (svcCtx *ServiceContext) GetCachedHotWalletState(ctx context.Context) (string, error) {
	if svcCtx == nil {
		return "", fmt.Errorf("service context is nil")
	}
	if svcCtx.SignerRpc == nil {
		return "", fmt.Errorf("signer rpc not available")
	}

	cache := svcCtx.hotWalletStateCache
	if cache == nil {
		st, err := svcCtx.SignerRpc.GetRuntimeStatus(ctx, &pb.GetRuntimeStatusRequest{SeedId: DefaultHotWalletSeedID})
		if err != nil || st == nil {
			return "", fmt.Errorf("failed to query signer runtime status")
		}
		return st.GetWalletState(), nil
	}

	now := time.Now()
	cache.mu.Lock()
	defer cache.mu.Unlock()

	if !cache.fetchedAt.IsZero() && now.Sub(cache.fetchedAt) < cache.ttl {
		return cache.state, nil
	}

	st, err := svcCtx.SignerRpc.GetRuntimeStatus(ctx, &pb.GetRuntimeStatusRequest{SeedId: DefaultHotWalletSeedID})
	if err != nil || st == nil {
		// Do not overwrite last known-good state on errors.
		logx.WithContext(ctx).Errorw("query signer runtime status failed", logx.Field("error", err))
		return "", fmt.Errorf("failed to query signer runtime status")
	}

	cache.state = st.GetWalletState()
	cache.fetchedAt = now
	return cache.state, nil
}
