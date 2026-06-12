package utils

import (
	"fmt"
	"strings"

	"internalwallet/proto/pb"
)

// ChainTypeToString 将 ChainRpcType 枚举转换为对应的字符串
func ChainTypeToString(chainType pb.ChainRpcType) string {
	switch chainType {
	case pb.ChainRpcType_CHAIN_TYPE_ETHEREUM:
		return "ETH"
	case pb.ChainRpcType_CHAIN_TYPE_BSC:
		return "BSC"
	case pb.ChainRpcType_CHAIN_TYPE_TRON:
		return "TRON"
	case pb.ChainRpcType_CHAIN_TYPE_POLYGON:
		return "POLYGON"
	default:
		return "UNKNOWN"
	}
}

// ChainTypeToFullName 将 ChainRpcType 枚举转换为完整的区块链名称
func ChainTypeToFullName(chainType pb.ChainRpcType) string {
	switch chainType {
	case pb.ChainRpcType_CHAIN_TYPE_ETHEREUM:
		return "Ethereum"
	case pb.ChainRpcType_CHAIN_TYPE_BSC:
		return "BNB Smart Chain"
	case pb.ChainRpcType_CHAIN_TYPE_TRON:
		return "Tron"
	case pb.ChainRpcType_CHAIN_TYPE_POLYGON:
		return "Polygon"
	default:
		return "Unknown Chain"
	}
}

// StringToChainType 将字符串转换为 ChainRpcType 枚举
func StringToChainType(chainStr string) (pb.ChainRpcType, error) {
	// 转换为大写并去除空格
	normalizedStr := strings.ToUpper(strings.TrimSpace(chainStr))

	switch normalizedStr {
	case "ETH", "ETHEREUM", "ETC":
		return pb.ChainRpcType_CHAIN_TYPE_ETHEREUM, nil
	case "BSC", "BINANCE", "BNB", "BNB SMART CHAIN":
		return pb.ChainRpcType_CHAIN_TYPE_BSC, nil
	case "TRON", "TRX", "TRC":
		return pb.ChainRpcType_CHAIN_TYPE_TRON, nil
	case "POLYGON", "MATIC", "POL":
		return pb.ChainRpcType_CHAIN_TYPE_POLYGON, nil
	default:
		return pb.ChainRpcType_CHAIN_TYPE_UNSPECIFIED, fmt.Errorf("unsupported chain type: %s", chainStr)
	}
}

// IsValidChainType 检查 ChainRpcType 是否有效
func IsValidChainType(chainType pb.ChainRpcType) bool {
	switch chainType {
	case pb.ChainRpcType_CHAIN_TYPE_ETHEREUM,
		pb.ChainRpcType_CHAIN_TYPE_BSC,
		pb.ChainRpcType_CHAIN_TYPE_TRON,
		pb.ChainRpcType_CHAIN_TYPE_POLYGON:
		return true
	default:
		return false
	}
}

// GetAllSupportedChains 返回所有支持的链类型
func GetAllSupportedChains() []pb.ChainRpcType {
	return []pb.ChainRpcType{
		pb.ChainRpcType_CHAIN_TYPE_ETHEREUM,
		pb.ChainRpcType_CHAIN_TYPE_BSC,
		pb.ChainRpcType_CHAIN_TYPE_TRON,
		pb.ChainRpcType_CHAIN_TYPE_POLYGON,
	}
}

// GetAllSupportedChainStrings 返回所有支持的链类型的字符串表示
func GetAllSupportedChainStrings() []string {
	chains := GetAllSupportedChains()
	strings := make([]string, len(chains))
	for i, chain := range chains {
		strings[i] = ChainTypeToString(chain)
	}
	return strings
}
