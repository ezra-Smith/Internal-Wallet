package logic

import (
	"math"
	"strconv"
	"strings"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/model"
)

func chainCodeToChainRpcType(chainCode string) pb.ChainRpcType {
	switch strings.ToUpper(strings.TrimSpace(chainCode)) {
	case "ETH", "ETHEREUM":
		return pb.ChainRpcType_CHAIN_TYPE_ETHEREUM
	case "BSC", "BNB", "BNB SMART CHAIN", "BINANCE", "BINANCE SMART CHAIN":
		return pb.ChainRpcType_CHAIN_TYPE_BSC
	case "TRON", "TRX":
		return pb.ChainRpcType_CHAIN_TYPE_TRON
	default:
		return pb.ChainRpcType_CHAIN_TYPE_UNSPECIFIED
	}
}

func chainCodeToNativeFeeAsset(chainCode string) string {
	switch strings.ToUpper(strings.TrimSpace(chainCode)) {
	case "ETH", "ETHEREUM":
		return "ETH"
	case "BSC", "BNB", "BNB SMART CHAIN", "BINANCE", "BINANCE SMART CHAIN":
		return "BNB"
	case "TRON", "TRX":
		return "TRX"
	default:
		return ""
	}
}

func chainRpcTxStatusToWeb3TxStatus(status pb.TxStatus) string {
	switch status {
	case pb.TxStatus_TX_STATUS_PENDING:
		return model.Web3TxStatusPending
	case pb.TxStatus_TX_STATUS_CONFIRMED:
		return model.Web3TxStatusConfirmed
	case pb.TxStatus_TX_STATUS_FAILED, pb.TxStatus_TX_STATUS_DROPPED, pb.TxStatus_TX_STATUS_REPLACED:
		return model.Web3TxStatusFailed
	default:
		return ""
	}
}

// deriveWeb3TxStatusFromChain derives UI-facing tx status using chain status + confirmations.
// We treat a tx as "confirmed" only when confirmations >= requiredConfirmations.
func deriveWeb3TxStatusFromChain(status pb.TxStatus, confirmations uint64, requiredConfirmations int32) string {
	s := chainRpcTxStatusToWeb3TxStatus(status)
	if s == "" {
		s = model.Web3TxStatusPending
	}
	if s != model.Web3TxStatusConfirmed {
		return s
	}
	if requiredConfirmations <= 0 {
		return s
	}
	if uint64ToInt32Clamp(confirmations) < requiredConfirmations {
		return model.Web3TxStatusPending
	}
	return s
}

func uint64ToInt32Clamp(v uint64) int32 {
	if v > math.MaxInt32 {
		return math.MaxInt32
	}
	return int32(v)
}

func parseInt64Flexible(s string) (int64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	var (
		v   int64
		err error
	)
	if strings.HasPrefix(strings.ToLower(s), "0x") {
		v, err = strconv.ParseInt(s[2:], 16, 64)
	} else {
		v, err = strconv.ParseInt(s, 10, 64)
	}
	if err != nil {
		return 0, false
	}
	return v, true
}

func parseBlockNumberPtr(s string) *int64 {
	v, ok := parseInt64Flexible(s)
	if !ok || v <= 0 {
		return nil
	}
	return &v
}

func blockTimestampToLocalTimePtr(ts int64) *time.Time {
	if ts <= 0 {
		return nil
	}
	// Some providers may return ms timestamps (especially TRON-like APIs).
	if ts > 1e12 {
		ts = ts / 1000
	}
	t := time.Unix(ts, 0).Local()
	return &t
}
