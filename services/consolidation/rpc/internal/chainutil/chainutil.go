package chainutil

import (
	"fmt"
	"strings"

	"internalwallet/proto/pb"
)

const (
	ChainTRON = "TRON"
	ChainETH  = "ETH"
	ChainBSC  = "BSC"
)

func SupportedChains() []string {
	return []string{ChainTRON, ChainETH, ChainBSC}
}

func NormalizeChain(chain string) string {
	return strings.ToUpper(strings.TrimSpace(chain))
}

func NormalizeAssetSymbol(asset string) string {
	return strings.ToUpper(strings.TrimSpace(asset))
}

func ChainStringToEnum(chain string) (pb.ChainRpcType, error) {
	switch NormalizeChain(chain) {
	case ChainTRON:
		return pb.ChainRpcType_CHAIN_TYPE_TRON, nil
	case ChainETH:
		return pb.ChainRpcType_CHAIN_TYPE_ETHEREUM, nil
	case ChainBSC:
		return pb.ChainRpcType_CHAIN_TYPE_BSC, nil
	default:
		return pb.ChainRpcType_CHAIN_TYPE_UNSPECIFIED, fmt.Errorf("unsupported chain: %s", chain)
	}
}

func NativeSymbolForChain(chain string) string {
	switch NormalizeChain(chain) {
	case ChainTRON:
		return "TRX"
	case ChainETH:
		return "ETH"
	case ChainBSC:
		return "BNB"
	default:
		return ""
	}
}

// NormalizeAddress normalizes an on-chain address for DB keys and comparisons.
// - EVM addresses are case-insensitive; use lower-case to ensure stable uniqueness.
// - TRON addresses are Base58 and case-sensitive; keep as trimmed input.
func NormalizeAddress(chain string, addr string) string {
	chain = NormalizeChain(chain)
	addr = strings.TrimSpace(addr)
	switch chain {
	case ChainETH, ChainBSC:
		return strings.ToLower(addr)
	default:
		return addr
	}
}

// NormalizeTokenContract normalizes a token contract address for DB keys and comparisons.
func NormalizeTokenContract(chain string, contract string) string {
	chain = NormalizeChain(chain)
	contract = strings.TrimSpace(contract)
	switch chain {
	case ChainETH, ChainBSC:
		return strings.ToLower(contract)
	default:
		return contract
	}
}
