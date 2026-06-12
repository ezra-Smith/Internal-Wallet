package svc

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/zeromicro/go-zero/core/logx"
)

// NonceManager 防止同一地址短时间内发送多笔交易时发生 nonce 冲突。
//
// 提供两个保障：
//  1. 按地址串行化：同一 (chain, address) 同时只允许一笔交易获取 nonce，
//     锁在调用方通过 release 函数通知完成后才释放。
//  2. 本地 nonce 追踪：广播成功后在本地记录已使用的 nonce，
//     下次获取时返回 max(链上 pendingNonce, 本地 lastNonce+1)，
//     避免 RPC 节点返回过时的 PendingNonceAt 结果。
type NonceManager struct {
	mu      sync.Mutex
	entries map[string]*nonceState
	ttl     time.Duration
}

type nonceState struct {
	mu        sync.Mutex // 按地址的串行化锁
	lastNonce uint64
	updatedAt time.Time
}

// NewNonceManager 创建 NonceManager。ttl 控制本地 nonce 条目的可信时长，
// 过期后将直接使用链上的 pending nonce。
func NewNonceManager(ttl time.Duration) *NonceManager {
	if ttl <= 0 {
		ttl = 2 * time.Minute
	}
	return &NonceManager{
		entries: make(map[string]*nonceState),
		ttl:     ttl,
	}
}

func (m *NonceManager) key(chain, address string) string {
	return fmt.Sprintf("%s:%s", strings.ToLower(chain), strings.ToLower(address))
}

func (m *NonceManager) getState(key string) *nonceState {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.entries[key]
	if !ok {
		s = &nonceState{}
		m.entries[key] = s
	}
	return s
}

// AcquireNonce 按 (chain, address) 串行获取 nonce。
//
// 先锁定该地址，再通过 ethClient 查询链上 pending nonce，
// 返回 max(pendingNonce, 本地 lastNonce+1)。
//
// 返回的 release 函数在交易处理完毕后 **必须** 调用：
//   - release(true)  — 记录该 nonce 已使用并释放锁
//   - release(false) — 直接释放锁，不记录（如广播失败时）
func (m *NonceManager) AcquireNonce(
	ctx context.Context,
	chain string,
	address string,
	ethClient *ethclient.Client,
) (nonce uint64, release func(success bool), err error) {
	k := m.key(chain, address)
	state := m.getState(k)

	// 串行化：阻塞直到上一笔该地址的交易调用 release() 释放锁。
	state.mu.Lock()

	addr := common.HexToAddress(address)
	pendingNonce, err := ethClient.PendingNonceAt(ctx, addr)
	if err != nil {
		state.mu.Unlock()
		return 0, nil, err
	}

	// 如果本地记录未过期且领先于链上值，使用本地 nonce。
	nonce = pendingNonce
	if !state.updatedAt.IsZero() && time.Since(state.updatedAt) < m.ttl && state.lastNonce+1 > pendingNonce {
		nonce = state.lastNonce + 1
		logx.Infof("[NonceManager] 本地 nonce=%d > 链上 pending=%d, 地址=%s, 链=%s, 使用本地值",
			nonce, pendingNonce, address, chain)
	} else {
		logx.Infof("[NonceManager] 使用链上 pending nonce=%d, 地址=%s, 链=%s", nonce, address, chain)
	}

	release = func(success bool) {
		if success {
			state.lastNonce = nonce
			state.updatedAt = time.Now()
		}
		state.mu.Unlock()
	}

	return nonce, release, nil
}

// Cleanup 清理过期的 nonce 条目，可定期调用。
func (m *NonceManager) Cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for k, s := range m.entries {
		if !s.updatedAt.IsZero() && now.Sub(s.updatedAt) > m.ttl*10 {
			delete(m.entries, k)
		}
	}
}
