package selfhosted

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
)

func (p *SelfHostedProvider) getEvmTransaction(ctx context.Context, txHash string) (*pb.Transaction, error) {
	resp, err := p.callRPC(ctx, "eth_getTransactionByHash", []interface{}{txHash})
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction %s: %v", txHash, err)
	}

	var tx map[string]interface{}
	if err := json.Unmarshal(resp.Result, &tx); err != nil {
		return nil, fmt.Errorf("failed to unmarshal transaction: %v", err)
	}

	if tx == nil {
		return nil, fmt.Errorf("transaction not found: %s", txHash)
	}

	receiptResp, err := p.callRPC(ctx, "eth_getTransactionReceipt", []interface{}{txHash})
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction receipt %s: %v", txHash, err)
	}

	var receipt map[string]interface{}
	if err := json.Unmarshal(receiptResp.Result, &receipt); err != nil {
		return nil, fmt.Errorf("failed to unmarshal receipt: %v", err)
	}

	result := &pb.Transaction{
		TxHash: txHash,
		Chain:  p.config.Chain,
	}

	if blockNumber, ok := receipt["blockNumber"].(string); ok {
		if bn, err := strconv.ParseInt(blockNumber[2:], 16, 64); err == nil {
			result.BlockNumber = uint64(bn)
		}
	}
	if blockHash, ok := receipt["blockHash"].(string); ok {
		result.BlockHash = blockHash
	}
	if txIndex, ok := receipt["transactionIndex"].(string); ok {
		if ti, err := strconv.ParseInt(txIndex[2:], 16, 64); err == nil {
			result.TransactionIndex = uint64(ti)
		}
	}
	if from, ok := tx["from"].(string); ok {
		result.FromAddress = from
	}
	if to, ok := tx["to"].(string); ok {
		result.ToAddress = to
	}
	if value, ok := tx["value"].(string); ok {
		result.Value = value
	}

	var gasPriceInt *big.Int
	if effectiveGasPrice, ok := receipt["effectiveGasPrice"].(string); ok && effectiveGasPrice != "" {
		result.GasPrice = effectiveGasPrice
		gasPriceInt = parseHexToBigInt(effectiveGasPrice)
	} else if gasPrice, ok := tx["gasPrice"].(string); ok {
		result.GasPrice = gasPrice
		gasPriceInt = parseHexToBigInt(gasPrice)
	}

	if gasUsed, ok := receipt["gasUsed"].(string); ok {
		if gu, err := strconv.ParseInt(gasUsed[2:], 16, 64); err == nil {
			result.GasUsed = uint64(gu)
		}
	}

	if gasPriceInt != nil && result.GasUsed > 0 {
		gasUsedInt := new(big.Int).SetUint64(result.GasUsed)
		gasFee := new(big.Int).Mul(gasPriceInt, gasUsedInt)
		result.GasFee = gasFee.String()
	}

	if nonce, ok := tx["nonce"].(string); ok {
		if n, err := strconv.ParseInt(nonce[2:], 16, 64); err == nil {
			result.Nonce = strconv.FormatInt(n, 10)
		}
	}

	if status, ok := receipt["status"].(string); ok {
		if status == "0x1" {
			result.Status = pb.TransactionStatus_TRANSACTION_STATUS_CONFIRMED
		} else {
			result.Status = pb.TransactionStatus_TRANSACTION_STATUS_FAILED
		}
	}

	if logsInterface, ok := receipt["logs"].([]interface{}); ok && len(logsInterface) > 0 {
		for _, logInterface := range logsInterface {
			logMap, ok := logInterface.(map[string]interface{})
			if !ok {
				continue
			}

			pbLog := &pb.TransactionLog{TransactionHash: txHash}

			if address, ok := logMap["address"].(string); ok {
				if strings.HasPrefix(address, "0x") && len(address) == 42 {
					addr := common.HexToAddress(address)
					pbLog.Address = addr.String()
				} else {
					pbLog.Address = address
				}
			}

			if topicsInterface, ok := logMap["topics"].([]interface{}); ok {
				for _, topicInterface := range topicsInterface {
					if topic, ok := topicInterface.(string); ok {
						pbLog.Topics = append(pbLog.Topics, topic)
					}
				}
			}

			if dataHex, ok := logMap["data"].(string); ok {
				if strings.HasPrefix(dataHex, "0x") {
					dataHex = dataHex[2:]
				}
				if decoded, err := hex.DecodeString(dataHex); err == nil {
					pbLog.Data = decoded
				}
			}

			if logIndexHex, ok := logMap["logIndex"].(string); ok {
				if logIndex, err := strconv.ParseInt(logIndexHex[2:], 16, 64); err == nil {
					pbLog.LogIndex = strconv.FormatInt(logIndex, 10)
				}
			}

			if blockNumberHex, ok := logMap["blockNumber"].(string); ok {
				if blockNum, err := strconv.ParseInt(blockNumberHex[2:], 16, 64); err == nil {
					pbLog.BlockNumber = uint64(blockNum)
				}
			}

			result.Logs = append(result.Logs, pbLog)
		}

		logx.Infof("📋 [ETH/BSC] Parsed %d logs for transaction %s", len(result.Logs), txHash)
	}

	if result.ToAddress != "" {
		internalTxs, err := p.getInternalTransactions(ctx, txHash)
		if err != nil {
			logx.Debugf("Failed to get internal transactions for %s: %v", txHash, err)
		} else if len(internalTxs) > 0 {
			result.InternalTransactions = internalTxs
			logx.Infof("🔄 [ETH/BSC] Parsed %d internal transactions for %s", len(internalTxs), txHash)
		}
	}

	return result, nil
}
