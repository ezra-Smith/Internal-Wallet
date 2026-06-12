package svc

import (
	"context"
	"fmt"
	"time"

	"internalwallet/proto/pb"
)

func (ar *AddressRegistry) FullRefresh(ctx context.Context) (addressRegistryRefreshResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if ar.loader == nil {
		return addressRegistryRefreshResult{}, fmt.Errorf("address loader is nil")
	}

	start := time.Now()
	builder := make(map[pb.BlockChainType]map[string]addressSourceBits)

	merge := func(chain pb.BlockChainType, addr string, src addressSourceBits) {
		if chain == pb.BlockChainType_CHAIN_TYPE_UNSPECIFIED {
			return
		}
		normalized := normalizeMonitoredAddress(chain, addr)
		if normalized == "" {
			return
		}
		m := builder[chain]
		if m == nil {
			m = make(map[string]addressSourceBits)
			builder[chain] = m
		}
		m[normalized] = m[normalized] | src
	}

	// 1) Deposit addresses (wallet_user_chain_addresses) via Admin RPC (active only).
	depositAddrs, _, err := ar.loader.LoadDepositAddresses(ctx, []string{}, 1, 500)
	if err != nil {
		return addressRegistryRefreshResult{}, fmt.Errorf("load deposit addresses failed: %w", err)
	}
	for _, a := range depositAddrs {
		if a == nil {
			continue
		}
		chainType := ar.loader.ConvertChainType(a.Chain)
		merge(chainType, a.Address, addressSourceDeposit)
	}

	// 2) Company wallets via Signer RPC (enabled only).
	companyWallets, err := ar.loader.LoadCompanyWallets(ctx, "", 0, 1, "")
	if err != nil {
		return addressRegistryRefreshResult{}, fmt.Errorf("load company wallets failed: %w", err)
	}
	for _, w := range companyWallets {
		if w == nil {
			continue
		}
		chainType := ar.loader.ConvertChainType(w.Chain)
		merge(chainType, w.Address, addressSourceCompany)
	}

	// 3) Vault addresses via Admin RPC (active only).
	vaultAddrs, _, err := ar.loader.LoadVaultAddresses(ctx, "active", 500)
	if err != nil {
		return addressRegistryRefreshResult{}, fmt.Errorf("load vault addresses failed: %w", err)
	}
	for _, a := range vaultAddrs {
		if a == nil {
			continue
		}
		chainType := ar.loader.ConvertChainType(a.Chain)
		merge(chainType, a.Address, addressSourceVault)
	}

	// 4) Web3 user addresses via Admin RPC (configurable).
	if ar.shouldMonitorWeb3() {
		web3Addrs, _, err := ar.loader.LoadWeb3UserAddresses(ctx, "enabled", 500)
		if err != nil {
			return addressRegistryRefreshResult{}, fmt.Errorf("load web3 addresses failed: %w", err)
		}
		for _, a := range web3Addrs {
			if a == nil {
				continue
			}
			chainType := ar.loader.ConvertChainType(a.Chain)
			merge(chainType, a.Address, addressSourceWeb3)
		}
	}

	// 5) Manual monitors from chainsync DB (supplementary only).
	if ar.repo != nil {
		monitors, err := ar.repo.GetActiveMonitors(ctx, "")
		if err != nil {
			return addressRegistryRefreshResult{}, fmt.Errorf("load manual monitors failed: %w", err)
		}
		for _, m := range monitors {
			if m == nil {
				continue
			}
			chainType := chainTypeFromString(m.Chain)
			merge(chainType, m.Address, addressSourceManual)
		}
	}

	applyDeltaToBuilder := func(builder map[pb.BlockChainType]map[string]addressSourceBits, delta *addressRegistryDelta) {
		if delta == nil {
			return
		}
		for chain, m := range delta.chains {
			if m == nil {
				continue
			}
			chainBuilder := builder[chain]
			if chainBuilder == nil {
				chainBuilder = make(map[string]addressSourceBits)
				builder[chain] = chainBuilder
			}
			for addr, d := range m {
				bits := chainBuilder[addr]
				bits = (bits | d.add) &^ d.remove
				if bits == 0 {
					delete(chainBuilder, addr)
					continue
				}
				chainBuilder[addr] = bits
			}
		}
	}

	// Swap snapshot atomically; keep lock narrow so event apply is not blocked by network calls.
	ar.mu.Lock()
	defer ar.mu.Unlock()

	prev := ar.Snapshot()
	applyDeltaToBuilder(builder, ar.getDelta())

	next := buildAddressRegistrySnapshot(builder, ar.shouldTreatWeb3AsInternal())
	diff := diffAddressRegistry(prev, next)

	now := time.Now()
	next.lastFullRefreshAt = now
	next.lastFullRefreshDuration = time.Since(start)
	next.lastFullRefreshAdded = diff.added
	next.lastFullRefreshRemoved = diff.removed

	// Preserve last event metadata for observability.
	if prev != nil {
		next.lastEventAppliedAt = prev.lastEventAppliedAt
		next.lastEventAppliedCount = prev.lastEventAppliedCount
		next.lastEventAppliedAdded = prev.lastEventAppliedAdded
		next.lastEventAppliedRemoved = prev.lastEventAppliedRemoved
		next.lastEventAppliedSource = prev.lastEventAppliedSource
		next.lastEventAppliedChain = prev.lastEventAppliedChain
		next.lastEventAppliedAddr = prev.lastEventAppliedAddr
		next.lastEventAppliedAction = prev.lastEventAppliedAction
		next.lastEventAppliedError = prev.lastEventAppliedError
		next.lastEventAppliedErrorAt = prev.lastEventAppliedErrorAt
		next.lastEventConsumerLagHint = prev.lastEventConsumerLagHint
	}

	ar.snapshot.Store(next)
	ar.delta.Store(newEmptyAddressRegistryDelta())

	return addressRegistryRefreshResult{added: diff.added, removed: diff.removed}, nil
}

func (ar *AddressRegistry) shouldMonitorWeb3() bool {
	if ar.cfg == nil {
		return true
	}
	// NOTE: go-zero config defaults do not reliably apply to nested optional structs.
	// Treat nil as "not configured" and default to true to avoid silently disabling web3 monitoring.
	if ar.cfg.Monitoring.AddressRegistry.MonitorWeb3Addresses == nil {
		return true
	}
	return *ar.cfg.Monitoring.AddressRegistry.MonitorWeb3Addresses
}

func (ar *AddressRegistry) shouldTreatWeb3AsInternal() bool {
	if ar.cfg == nil {
		return false
	}
	return ar.cfg.Monitoring.AddressRegistry.TreatWeb3AsInternal
}
