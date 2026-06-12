package svc

import (
	"math"
	"math/big"
	"strings"

	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
)

// parseInternalTransactions 解析内部交易中的 ETH 转账
func (tp *TokenParser) parseInternalTransactions(tx *pb.Transaction) []*TokenTransaction {
	var results []*TokenTransaction

	for _, internalTx := range tx.InternalTransactions {
		// 跳过失败的内部交易
		if internalTx.Error != "" {
			continue
		}

		// 跳过 value = 0 的内部交易
		if internalTx.Value == "" || internalTx.Value == "0" || internalTx.Value == "0x0" {
			continue
		}

		// 检查是否与监控地址相关
		isRelevant := tp.isAddressRelevant(tx.Chain, internalTx.FromAddress, internalTx.ToAddress)
		if !isRelevant {
			continue
		}

		weiDecimal := convertHexWeiToDecimal(internalTx.Value)
		formattedValue := formatWeiToEther(internalTx.Value)
		eventIndex := internalTransferEventIndex(internalTx.TraceAddressIndex)

		nativeTx := &TokenTransaction{
			TxHash:          tx.TxHash,
			Chain:           tx.Chain,
			BlockNumber:     tx.BlockNumber,
			From:            internalTx.FromAddress,
			To:              internalTx.ToAddress,
			Status:          uint8(tx.Status),
			Timestamp:       tx.BlockTimestamp,
			EventIndex:      eventIndex,
			TokenAmount:     weiDecimal,
			TokenValue:      formattedValue,
			TransactionType: "native",
		}

		results = append(results, nativeTx)
		logx.Debugf("🔄 Internal tx in %s: %s → %s, amount=%s wei, value=%s ETH",
			tx.TxHash, internalTx.FromAddress, internalTx.ToAddress, weiDecimal, formattedValue)
	}

	return results
}

func internalTransferEventIndex(traceIndex uint64) int32 {
	const offset int64 = 1_000_000_000
	if traceIndex > uint64(math.MaxInt32-offset) {
		// Too large for int32; clamp to MaxInt32 to avoid overflow (dedup may degrade).
		return math.MaxInt32
	}
	return int32(offset + int64(traceIndex))
}

// convertHexWeiToDecimal 将 16 进制 Wei 转换为十进制字符串
func convertHexWeiToDecimal(weiStr string) string {
	if weiStr == "" || weiStr == "0" || weiStr == "0x0" {
		return "0"
	}

	wei := new(big.Int)

	if strings.HasPrefix(weiStr, "0x") {
		hexStr := weiStr[2:]
		if _, success := wei.SetString(hexStr, 16); !success {
			logx.Errorf("Failed to parse hex wei value: %s", weiStr)
			return "0"
		}
	} else {
		if _, success := wei.SetString(weiStr, 10); !success {
			logx.Errorf("Failed to parse decimal wei value: %s", weiStr)
			return "0"
		}
	}

	return wei.String()
}

// formatWeiToEther 将 Wei 转换为 ETH（完整精度，不做四舍五入）
func formatWeiToEther(weiStr string) string {
	if strings.HasPrefix(weiStr, "0x") {
		weiStr = weiStr[2:]
	}

	wei := new(big.Int)
	if weiStr == "" {
		return "0"
	}

	if _, success := wei.SetString(weiStr, 16); !success {
		if _, success = wei.SetString(weiStr, 10); !success {
			logx.Errorf("Failed to parse wei value: %s", weiStr)
			return "0"
		}
	}

	divisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)

	quotient := new(big.Int).Div(wei, divisor)
	remainder := new(big.Int).Mod(wei, divisor)

	if remainder.Cmp(big.NewInt(0)) == 0 {
		return quotient.String()
	}

	remainderStr := remainder.String()
	for len(remainderStr) < 18 {
		remainderStr = "0" + remainderStr
	}

	remainderStr = strings.TrimRight(remainderStr, "0")
	if remainderStr == "" {
		return quotient.String()
	}

	return quotient.String() + "." + remainderStr
}
