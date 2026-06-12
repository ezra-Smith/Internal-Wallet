package svc

import (
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/chainsync/rpc/internal/config"
	"internalwallet/services/chainsync/rpc/internal/repository"
)

type addressSourceBits uint8

const (
	addressSourceDeposit addressSourceBits = 1 << iota
	addressSourceCompany
	addressSourceVault
	addressSourceWeb3
	addressSourceManual
)

type chainSnapshot struct {
	addresses         []string
	sourceBitsByAddr  map[string]addressSourceBits
	total             int
	depositCount      int
	companyCount      int
	vaultCount        int
	web3Count         int
	manualCount       int
	monitoredCount    int
	internalCount     int // deposit+company+vault (+web3 if configured as internal)
	internalWeb3Count int // web3 that is counted as internal (usually 0)
}

type addressRegistrySnapshot struct {
	chains map[pb.BlockChainType]*chainSnapshot

	lastFullRefreshAt        time.Time
	lastFullRefreshDuration  time.Duration
	lastFullRefreshAdded     int
	lastFullRefreshRemoved   int
	lastEventAppliedAt       time.Time
	lastEventAppliedCount    int
	lastEventAppliedAdded    int
	lastEventAppliedRemoved  int
	lastEventAppliedSource   string
	lastEventAppliedChain    pb.BlockChainType
	lastEventAppliedAddr     string
	lastEventAppliedAction   string
	lastEventAppliedError    string
	lastEventAppliedErrorAt  time.Time
	lastEventConsumerLagHint string
}

func newEmptyAddressRegistrySnapshot() *addressRegistrySnapshot {
	return &addressRegistrySnapshot{
		chains: map[pb.BlockChainType]*chainSnapshot{
			pb.BlockChainType_CHAIN_TYPE_ETHEREUM: {sourceBitsByAddr: map[string]addressSourceBits{}},
			pb.BlockChainType_CHAIN_TYPE_BSC:      {sourceBitsByAddr: map[string]addressSourceBits{}},
			pb.BlockChainType_CHAIN_TYPE_TRON:     {sourceBitsByAddr: map[string]addressSourceBits{}},
		},
	}
}

// AddressRegistry maintains the monitored address sets (by source) as an immutable snapshot
// swapped atomically. Full refresh is the source-of-truth, event updates are incremental.
type AddressRegistry struct {
	cfg    *config.Config
	loader *AddressLoader
	repo   repository.AddressMonitorRepository

	snapshot atomic.Value // *addressRegistrySnapshot (full refresh snapshot)
	delta    atomic.Value // *addressRegistryDelta (event-driven incremental updates)
	mu       sync.Mutex   // serialize refresh / apply
}

func NewAddressRegistry(cfg *config.Config, loader *AddressLoader, repo repository.AddressMonitorRepository) *AddressRegistry {
	ar := &AddressRegistry{
		cfg:    cfg,
		loader: loader,
		repo:   repo,
	}
	ar.snapshot.Store(newEmptyAddressRegistrySnapshot())
	ar.delta.Store(newEmptyAddressRegistryDelta())
	return ar
}

func (ar *AddressRegistry) Snapshot() *addressRegistrySnapshot {
	v := ar.snapshot.Load()
	if v == nil {
		return newEmptyAddressRegistrySnapshot()
	}
	s, ok := v.(*addressRegistrySnapshot)
	if !ok || s == nil {
		return newEmptyAddressRegistrySnapshot()
	}
	return s
}

type addressRegistryRefreshResult struct {
	added   int
	removed int
}

func buildAddressRegistrySnapshot(builder map[pb.BlockChainType]map[string]addressSourceBits, treatWeb3AsInternal bool) *addressRegistrySnapshot {
	out := newEmptyAddressRegistrySnapshot()
	out.chains = make(map[pb.BlockChainType]*chainSnapshot, 3)

	for _, chain := range []pb.BlockChainType{
		pb.BlockChainType_CHAIN_TYPE_ETHEREUM,
		pb.BlockChainType_CHAIN_TYPE_BSC,
		pb.BlockChainType_CHAIN_TYPE_TRON,
	} {
		m := builder[chain]
		if m == nil {
			m = make(map[string]addressSourceBits)
		}

		addrs := make([]string, 0, len(m))
		for addr := range m {
			addrs = append(addrs, addr)
		}
		// Deterministic ordering keeps pagination stable and simplifies debugging.
		sort.Strings(addrs)

		cs := &chainSnapshot{
			addresses:        addrs,
			sourceBitsByAddr: m,
			total:            len(addrs),
		}

		for _, addr := range addrs {
			bits := m[addr]
			if bits == 0 {
				continue
			}
			cs.monitoredCount++
			if bits&addressSourceDeposit != 0 {
				cs.depositCount++
				cs.internalCount++
			}
			if bits&addressSourceCompany != 0 {
				cs.companyCount++
				cs.internalCount++
			}
			if bits&addressSourceVault != 0 {
				cs.vaultCount++
				cs.internalCount++
			}
			if bits&addressSourceWeb3 != 0 {
				cs.web3Count++
				if treatWeb3AsInternal {
					cs.internalCount++
					cs.internalWeb3Count++
				}
			}
			if bits&addressSourceManual != 0 {
				cs.manualCount++
			}
		}

		out.chains[chain] = cs
	}
	return out
}

type addressRegistryDiff struct {
	added   int
	removed int
}

func diffAddressRegistry(prev *addressRegistrySnapshot, next *addressRegistrySnapshot) addressRegistryDiff {
	if prev == nil || next == nil {
		return addressRegistryDiff{}
	}
	added := 0
	removed := 0

	for chain, nextChain := range next.chains {
		var prevMap map[string]addressSourceBits
		if prevChain := prev.chains[chain]; prevChain != nil {
			prevMap = prevChain.sourceBitsByAddr
		}
		if prevMap == nil {
			prevMap = map[string]addressSourceBits{}
		}
		for addr := range nextChain.sourceBitsByAddr {
			if _, ok := prevMap[addr]; !ok {
				added++
			}
		}
		for addr := range prevMap {
			if _, ok := nextChain.sourceBitsByAddr[addr]; !ok {
				removed++
			}
		}
	}
	return addressRegistryDiff{added: added, removed: removed}
}

func chainTypeFromString(chainStr string) pb.BlockChainType {
	s := strings.TrimSpace(chainStr)
	if s == "" {
		return pb.BlockChainType_CHAIN_TYPE_UNSPECIFIED
	}
	switch strings.ToUpper(s) {
	case "ETH", "ETHEREUM", "ETH_MAINNET", "ETHEREUM_MAINNET", "ETHEREUM-SEPOLIA", "SEPOLIA":
		return pb.BlockChainType_CHAIN_TYPE_ETHEREUM
	case "BSC", "BNB", "BNBCHAIN", "BNB SMART CHAIN":
		return pb.BlockChainType_CHAIN_TYPE_BSC
	case "TRN", "TRON", "TRX":
		return pb.BlockChainType_CHAIN_TYPE_TRON
	default:
		return pb.BlockChainType_CHAIN_TYPE_UNSPECIFIED
	}
}

func chainStringForDB(chain pb.BlockChainType) string {
	switch chain {
	case pb.BlockChainType_CHAIN_TYPE_ETHEREUM:
		return "ethereum"
	case pb.BlockChainType_CHAIN_TYPE_BSC:
		return "bsc"
	case pb.BlockChainType_CHAIN_TYPE_TRON:
		return "tron"
	default:
		return ""
	}
}

type addressDelta struct {
	add    addressSourceBits
	remove addressSourceBits
}

type addressRegistryDelta struct {
	chains      map[pb.BlockChainType]map[string]addressDelta
	lastApplied time.Time
}

func newEmptyAddressRegistryDelta() *addressRegistryDelta {
	return &addressRegistryDelta{
		chains: map[pb.BlockChainType]map[string]addressDelta{
			pb.BlockChainType_CHAIN_TYPE_ETHEREUM: {},
			pb.BlockChainType_CHAIN_TYPE_BSC:      {},
			pb.BlockChainType_CHAIN_TYPE_TRON:     {},
		},
	}
}

func (ar *AddressRegistry) getDelta() *addressRegistryDelta {
	v := ar.delta.Load()
	if v == nil {
		return newEmptyAddressRegistryDelta()
	}
	d, ok := v.(*addressRegistryDelta)
	if !ok || d == nil {
		return newEmptyAddressRegistryDelta()
	}
	return d
}

func (ar *AddressRegistry) IsMonitored(chain pb.BlockChainType, addr string) bool {
	chain = normalizeChainType(chain)
	if chain == pb.BlockChainType_CHAIN_TYPE_UNSPECIFIED {
		return false
	}
	normalized := normalizeMonitoredAddress(chain, addr)
	if normalized == "" {
		return false
	}

	snap := ar.Snapshot()
	delta := ar.getDelta()
	return ar.effectiveBits(chain, normalized, snap, delta) != 0
}

func (ar *AddressRegistry) IsInternal(chain pb.BlockChainType, addr string) bool {
	chain = normalizeChainType(chain)
	if chain == pb.BlockChainType_CHAIN_TYPE_UNSPECIFIED {
		return false
	}
	normalized := normalizeMonitoredAddress(chain, addr)
	if normalized == "" {
		return false
	}

	snap := ar.Snapshot()
	delta := ar.getDelta()
	bits := ar.effectiveBits(chain, normalized, snap, delta)
	if bits&(addressSourceDeposit|addressSourceCompany|addressSourceVault) != 0 {
		return true
	}
	if ar.shouldTreatWeb3AsInternal() && bits&addressSourceWeb3 != 0 {
		return true
	}
	return false
}

func normalizeChainType(chain pb.BlockChainType) pb.BlockChainType {
	switch chain {
	case pb.BlockChainType_CHAIN_TYPE_ETHEREUM, pb.BlockChainType_CHAIN_TYPE_BSC, pb.BlockChainType_CHAIN_TYPE_TRON:
		return chain
	default:
		return pb.BlockChainType_CHAIN_TYPE_UNSPECIFIED
	}
}

func (ar *AddressRegistry) effectiveBits(chain pb.BlockChainType, normalizedAddr string, snap *addressRegistrySnapshot, delta *addressRegistryDelta) addressSourceBits {
	var baseBits addressSourceBits
	if snap != nil {
		if cs := snap.chains[chain]; cs != nil && cs.sourceBitsByAddr != nil {
			baseBits = cs.sourceBitsByAddr[normalizedAddr]
		}
	}

	if delta == nil {
		return baseBits
	}
	if m := delta.chains[chain]; m != nil {
		if d, ok := m[normalizedAddr]; ok {
			baseBits = (baseBits | d.add) &^ d.remove
		}
	}
	return baseBits
}

// BuildMonitoredAddressSet returns a per-chain monitored address set for block scanning.
// It applies the incremental delta (event-driven) on top of the last full refresh snapshot.
func (ar *AddressRegistry) BuildMonitoredAddressSet(chain pb.BlockChainType) map[string]bool {
	chain = normalizeChainType(chain)
	if chain == pb.BlockChainType_CHAIN_TYPE_UNSPECIFIED {
		return map[string]bool{}
	}

	snap := ar.Snapshot()
	delta := ar.getDelta()

	base := snap.chains[chain]
	if base == nil {
		base = &chainSnapshot{addresses: nil, sourceBitsByAddr: map[string]addressSourceBits{}}
	}

	// Pre-size: base addresses + delta entries (best-effort)
	capHint := len(base.addresses)
	if deltaChain := delta.chains[chain]; deltaChain != nil {
		capHint += len(deltaChain)
	}
	out := make(map[string]bool, capHint)

	// 1) Base addresses (apply delta)
	for _, addr := range base.addresses {
		if ar.effectiveBits(chain, addr, snap, delta) != 0 {
			out[addr] = true
		}
	}

	// 2) Delta-only addresses (not in base)
	if deltaChain := delta.chains[chain]; deltaChain != nil {
		for addr := range deltaChain {
			if base.sourceBitsByAddr != nil {
				if _, inBase := base.sourceBitsByAddr[addr]; inBase {
					continue
				}
			}
			if ar.effectiveBits(chain, addr, snap, delta) != 0 {
				out[addr] = true
			}
		}
	}

	return out
}
