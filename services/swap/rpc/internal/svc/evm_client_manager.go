package svc

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"

	"internalwallet/services/swap/rpc/internal/config"
)

type EVMClientManager struct {
	mu      sync.RWMutex
	chains  map[int64]config.ChainConfig
	clients map[int64]*ethclient.Client
}

func NewEVMClientManager(chains []config.ChainConfig) *EVMClientManager {
	m := &EVMClientManager{
		chains:  make(map[int64]config.ChainConfig, len(chains)),
		clients: make(map[int64]*ethclient.Client),
	}
	for _, c := range chains {
		if c.ChainID == 0 {
			continue
		}
		m.chains[c.ChainID] = c
	}
	return m
}

func (m *EVMClientManager) GetChain(chainID int64) (config.ChainConfig, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.chains[chainID]
	return c, ok
}

func (m *EVMClientManager) GetRequiredConfirmations(chainID int64) (uint64, bool) {
	c, ok := m.GetChain(chainID)
	if !ok {
		return 0, false
	}
	if c.RequiredConfirmations <= 0 {
		return 12, true
	}
	return c.RequiredConfirmations, true
}

func (m *EVMClientManager) GetClient(ctx context.Context, chainID int64) (*ethclient.Client, error) {
	m.mu.RLock()
	if cli, ok := m.clients[chainID]; ok && cli != nil {
		m.mu.RUnlock()
		return cli, nil
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	// re-check after acquiring lock
	if cli, ok := m.clients[chainID]; ok && cli != nil {
		return cli, nil
	}

	chainCfg, ok := m.chains[chainID]
	if !ok || !chainCfg.Enabled {
		return nil, fmt.Errorf("unsupported chain_id: %d", chainID)
	}

	endpoints := make([]string, 0, len(chainCfg.RPCEndpoints))
	for _, e := range chainCfg.RPCEndpoints {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		endpoints = append(endpoints, e)
	}
	if len(endpoints) == 0 {
		return nil, fmt.Errorf("chain rpc_endpoints not configured for chain_id: %d", chainID)
	}

	var lastErr error
	for _, endpoint := range endpoints {
		dctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		cli, err := ethclient.DialContext(dctx, endpoint)
		cancel()
		if err == nil {
			m.clients[chainID] = cli
			return cli, nil
		}
		lastErr = err
	}

	return nil, fmt.Errorf("failed to dial evm rpc (chain_id=%d): %w", chainID, lastErr)
}
