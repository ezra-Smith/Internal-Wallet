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

	"internalwallet/proto/pb"
	"internalwallet/services/chainsync/rpc/internal/units"
)

func (p *SelfHostedProvider) getTransactionReceipt(ctx context.Context, txHash string) (*pb.Transaction, error) {
	rpcResp, err := p.callRPC(ctx, "eth_getTransactionReceipt", []interface{}{txHash})
	if err != nil {
		return nil, fmt.Errorf("failed to get receipt for %s: %v", txHash, err)
	}

	if rpcResp.Result == nil {
		return nil, fmt.Errorf("receipt not found for %s", txHash)
	}

	var receipt struct {
		BlockHash         string                   `json:"blockHash"`
		BlockNumber       string                   `json:"blockNumber"`
		TransactionIndex  string                   `json:"transactionIndex"`
		GasUsed           string                   `json:"gasUsed"`
		EffectiveGasPrice string                   `json:"effectiveGasPrice,omitempty"`
		Status            string                   `json:"status"`
		Logs              []map[string]interface{} `json:"logs"`
	}

	if err := json.Unmarshal(rpcResp.Result, &receipt); err != nil {
		return nil, fmt.Errorf("failed to unmarshal receipt: %v", err)
	}

	logs := make([]*pb.TransactionLog, 0, len(receipt.Logs))
	for _, logMap := range receipt.Logs {
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

		logs = append(logs, pbLog)
	}

	var gasUsed uint64
	if receipt.GasUsed != "" {
		if gu, err := strconv.ParseUint(receipt.GasUsed[2:], 16, 64); err == nil {
			gasUsed = gu
		}
	}

	effectiveGasPrice := "0x0"
	if receipt.EffectiveGasPrice != "" {
		effectiveGasPrice = receipt.EffectiveGasPrice
	}

	txStatus := pb.TransactionStatus_TRANSACTION_STATUS_PENDING
	if receipt.Status != "" {
		if receipt.Status == "0x1" {
			txStatus = pb.TransactionStatus_TRANSACTION_STATUS_CONFIRMED
		} else if receipt.Status == "0x0" {
			txStatus = pb.TransactionStatus_TRANSACTION_STATUS_FAILED
		}
	}

	pbTx := &pb.Transaction{
		TxHash:  txHash,
		Logs:    logs,
		GasUsed: gasUsed,
		Status:  txStatus,
	}

	if effectiveGasPrice != "0x0" && gasUsed > 0 {
		gasPriceInt := parseHexToBigInt(effectiveGasPrice)
		if gasPriceInt != nil {
			gasUsedInt := new(big.Int).SetUint64(gasUsed)
			gasFee := new(big.Int).Mul(gasPriceInt, gasUsedInt)
			pbTx.GasFee = gasFee.String()
			pbTx.GasPrice = units.FormatTokenAmount(gasPriceInt.String(), 9)
		}
	}

	return pbTx, nil
}

