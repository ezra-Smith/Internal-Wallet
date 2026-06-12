package svc

import (
	"math/big"
	"strings"

	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
)

// parseTRC10Transaction 解析 TRC-10 代币交易（TRON 原生代币协议）
func (tp *TokenParser) parseTRC10Transaction(tx *pb.Transaction) *TokenTransaction {
	if tx.ContractAddress != "" || len(tx.Logs) > 0 {
		logx.Debugf("🚫 Not TRC-10 (has ContractAddress or Logs): %s", tx.TxHash)
		return nil
	}

	if tx.Value == "" || !strings.Contains(tx.Value, " ") {
		return nil
	}

	parts := strings.SplitN(tx.Value, " ", 2)
	if len(parts) != 2 {
		return nil
	}

	amount := strings.TrimSpace(parts[0])
	tokenSymbol := strings.TrimSpace(parts[1])

	if tokenSymbol == "" || strings.ToUpper(tokenSymbol) == "TRX" {
		logx.Debugf("🚫 Skipping TRX native transfer: %s", tx.Value)
		return nil
	}

	if _, ok := new(big.Int).SetString(amount, 10); !ok {
		logx.Debugf("⚠️ Invalid amount format in TRC-10: %s", amount)
		return nil
	}

	if !tp.isAddressRelevant(tx.Chain, tx.FromAddress, tx.ToAddress) {
		return nil
	}

	tokenTx := &TokenTransaction{
		TxHash:          tx.TxHash,
		Chain:           tx.Chain,
		BlockNumber:     tx.BlockNumber,
		From:            tx.FromAddress,
		To:              tx.ToAddress,
		Status:          uint8(tx.Status),
		Timestamp:       tx.BlockTimestamp,
		EventIndex:      0,
		TokenSymbol:     tokenSymbol,
		TokenName:       tokenSymbol,
		TokenDecimals:   6,
		TokenAmount:     amount,
		TokenValue:      tp.calculateTRC10Value(amount, 6),
		TransactionType: "token",
	}

	logx.Infof("🪙 TRC-10 Transfer: %s → %s, amount: %s %s",
		tx.FromAddress, tx.ToAddress, amount, tokenSymbol)

	return tokenTx
}

// calculateTRC10Value 计算 TRC-10 代币显示值
func (tp *TokenParser) calculateTRC10Value(amount string, decimals uint8) string {
	bigAmount, ok := new(big.Int).SetString(amount, 10)
	if !ok {
		return amount
	}
	return tp.calculateTokenValue(bigAmount, decimals)
}
