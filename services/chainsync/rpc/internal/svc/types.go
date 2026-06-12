package svc

import "internalwallet/services/chainsync/rpc/internal/provider"

// Provider is the blockchain provider interface used by chainsync.
type Provider = provider.Provider

// TokenInfo describes an on-chain token contract.
type TokenInfo struct {
	Address      string `json:"address"`
	Symbol       string `json:"symbol"`
	Name         string `json:"name"`
	Decimals     uint8  `json:"decimals"`
	Chain        string `json:"chain"`
	IsStablecoin bool   `json:"is_stablecoin"`
}

// TokenInfoQueryFunc is used to share token info lookup across components.
type TokenInfoQueryFunc func(chain, contractAddress string) *TokenInfo
