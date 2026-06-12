package svc

import (
	"strings"
	"sync"
	"time"
)

// WalletRuntime 保存 signer 进程内的解锁运行态（不会持久化）。
// - 未解锁时：不应允许任何签名/派生等敏感操作
// - 解锁后：seed 仅驻留内存；进程重启即丢失，需要重新解锁
type WalletRuntime struct {
	mu sync.RWMutex

	// seedID -> seedBytes
	seeds map[string][]byte

	// 仅用于诊断，不包含敏感信息
	unlockedAt *time.Time
}

func NewWalletRuntime() *WalletRuntime {
	return &WalletRuntime{
		seeds: make(map[string][]byte),
	}
}

func (r *WalletRuntime) IsSeedUnlocked(seedID string) bool {
	seedID = strings.TrimSpace(seedID)
	if seedID == "" {
		return false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.seeds[seedID]
	return ok
}

func (r *WalletRuntime) GetSeedCopy(seedID string) ([]byte, bool) {
	seedID = strings.TrimSpace(seedID)
	if seedID == "" {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.seeds[seedID]
	if !ok || len(v) == 0 {
		return nil, false
	}
	out := make([]byte, len(v))
	copy(out, v)
	return out, true
}

func (r *WalletRuntime) SetSeed(seedID string, seed []byte, at time.Time) {
	seedID = strings.TrimSpace(seedID)
	if seedID == "" || len(seed) == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := make([]byte, len(seed))
	copy(cp, seed)
	r.seeds[seedID] = cp
	r.unlockedAt = &at
}

func (r *WalletRuntime) LockAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for k := range r.seeds {
		delete(r.seeds, k)
	}
	r.unlockedAt = nil
}

func (r *WalletRuntime) UnlockedAt() *time.Time {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.unlockedAt == nil {
		return nil
	}
	t := *r.unlockedAt
	return &t
}
