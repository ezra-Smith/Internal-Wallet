package selfhosted

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
	"internalwallet/services/chainsync/rpc/internal/provider"
)

func (p *SelfHostedProvider) getEthLikeBlockTransactions(ctx context.Context, blockNumber uint64) ([]*pb.Transaction, error) {
	params := []interface{}{fmt.Sprintf("0x%x", blockNumber), true}

	rpcResp, err := p.callRPC(ctx, "eth_getBlockByNumber", params)
	if err != nil {
		return nil, fmt.Errorf("failed to get block %d: %v", blockNumber, err)
	}

	var blockData struct {
		Hash         string                   `json:"hash"`
		ParentHash   string                   `json:"parentHash"`
		Number       string                   `json:"number"`
		Timestamp    string                   `json:"timestamp"`
		Transactions []map[string]interface{} `json:"transactions"`
	}

	if err := json.Unmarshal(rpcResp.Result, &blockData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal block response: %v", err)
	}

	skipReceipts := len(blockData.Transactions) > 200 || provider.ShouldSkipReceipts(ctx)

	logx.Infof("📦📦 [%s] Block %d contains %d transactions (hash: %s)",
		p.config.Type, blockNumber, len(blockData.Transactions), blockData.Hash)

	var blockTimestamp int64
	if blockData.Timestamp != "" {
		if tsUint64, err := strconv.ParseUint(blockData.Timestamp, 0, 64); err == nil {
			blockTimestamp = int64(tsUint64)
		}
	}

	transactions := make([]*pb.Transaction, 0, len(blockData.Transactions))
	for _, tx := range blockData.Transactions {
		pbTx := p.convertRPCTransactionToPbTransaction(tx, blockNumber, blockTimestamp)

		if !skipReceipts {
			if txHash, ok := tx["hash"].(string); ok {
				receipt, err := p.getTransactionReceipt(ctx, txHash)
				if err == nil && receipt != nil {
					pbTx.Status = receipt.Status
					if receipt.GasUsed > 0 {
						pbTx.GasUsed = receipt.GasUsed
					}
					if receipt.GasFee != "" {
						pbTx.GasFee = receipt.GasFee
					}
					if receipt.GasPrice != "" {
						pbTx.GasPrice = receipt.GasPrice
					}
					if len(receipt.Logs) > 0 {
						pbTx.Logs = receipt.Logs
					}
				} else {
					logx.Infof("⚠️ [%s] Failed to get receipt for tx %s: %v, keeping tx as pending", p.config.Type, txHash, err)
				}
			}
		}

		transactions = append(transactions, pbTx)
	}

	return transactions, nil
}

func (p *SelfHostedProvider) isComplexTransaction(tx map[string]interface{}) bool {
	to, ok := tx["to"].(string)
	if !ok || to == "" {
		return true
	}

	input, ok := tx["input"].(string)
	if !ok || len(input) < 10 {
		return false
	}

	methodSig := input[:10]
	if methodSig == "0xa9059cbb" {
		return false
	}

	return true
}
