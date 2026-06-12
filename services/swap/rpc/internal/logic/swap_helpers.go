package logic

import (
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"internalwallet/proto/pb"
	"internalwallet/services/swap/rpc/internal/provider"
)

const nativeTokenAddress = "0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE"

func normalizeEVMAddress(addr string) string {
	return strings.ToLower(strings.TrimSpace(addr))
}

func isValidWalletAddress(addr string) bool {
	addr = strings.TrimSpace(addr)
	return common.IsHexAddress(addr)
}

func isValidTokenAddress(addr string) bool {
	addr = strings.TrimSpace(addr)
	if strings.EqualFold(addr, nativeTokenAddress) {
		return true
	}
	return common.IsHexAddress(addr)
}

func parseUintDecimal(s string) (*big.Int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty amount")
	}
	n := new(big.Int)
	if _, ok := n.SetString(s, 10); !ok {
		return nil, fmt.Errorf("invalid amount")
	}
	if n.Sign() < 0 {
		return nil, fmt.Errorf("invalid amount")
	}
	return n, nil
}

func providerTokenToPB(t provider.Token) *pb.SwapTokenInfo {
	return &pb.SwapTokenInfo{
		Address:  t.Address,
		Symbol:   t.Symbol,
		Name:     t.Name,
		Decimals: t.Decimals,
		LogoUri:  t.LogoURI,
	}
}

func providerQuoteToPB(q *provider.QuoteResponse) *pb.QuoteData {
	if q == nil {
		return nil
	}
	return &pb.QuoteData{
		ChainId:      q.ChainID,
		FromToken:    providerTokenToPB(q.FromToken),
		ToToken:      providerTokenToPB(q.ToToken),
		FromAmount:   q.FromAmount,
		ToAmount:     q.ToAmount,
		EstimatedGas: q.EstimatedGas,
		Provider:     q.Provider,
		TradeFee:     q.TradeFee,
		PriceImpact:  q.PriceImpact,
	}
}

func providerTxToPB(tx *provider.UnsignedTransaction) *pb.UnsignedTransaction {
	if tx == nil {
		return nil
	}
	return &pb.UnsignedTransaction{
		ChainId:  tx.ChainID,
		From:     tx.From,
		To:       tx.To,
		Data:     tx.Data,
		Value:    tx.Value,
		Gas:      tx.Gas,
		GasPrice: tx.GasPrice,
	}
}

func formatRFC3339(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func mapStatusToProto(status string) pb.SwapTxStatus {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "created":
		return pb.SwapTxStatus_SWAP_TX_STATUS_CREATED
	case "pending":
		return pb.SwapTxStatus_SWAP_TX_STATUS_PENDING
	case "success":
		return pb.SwapTxStatus_SWAP_TX_STATUS_SUCCESS
	case "failed":
		return pb.SwapTxStatus_SWAP_TX_STATUS_FAILED
	default:
		return pb.SwapTxStatus_SWAP_TX_STATUS_UNSPECIFIED
	}
}
