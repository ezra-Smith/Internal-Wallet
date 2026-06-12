package logic

import (
	"strings"

	"internalwallet/proto/pb"
)

func mapChainType(code string) pb.ChainRpcType {
	switch strings.ToUpper(strings.TrimSpace(code)) {
	case "TRN", "TRON", "TRX":
		return pb.ChainRpcType_CHAIN_TYPE_TRON
	case "BSC", "BNB":
		return pb.ChainRpcType_CHAIN_TYPE_BSC
	case "ETH", "ETHEREUM":
		return pb.ChainRpcType_CHAIN_TYPE_ETHEREUM
	case "POLYGON", "MATIC":
		return pb.ChainRpcType_CHAIN_TYPE_POLYGON
	default:
		return pb.ChainRpcType_CHAIN_TYPE_UNSPECIFIED
	}
}
