package evm

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
	"internalwallet/services/chainsync/rpc/internal/provider"
	"internalwallet/services/chainsync/rpc/internal/units"
)

func (p *Web3Provider) GetBlockTransactions(ctx context.Context, blockNumber uint64) ([]*pb.Transaction, error) {
	start := time.Now()
	var err error
	defer p.finishRequest(start, &err)

	if p.client == nil {
		err = fmt.Errorf("ethereum client is nil")
		return nil, err
	}

	block, err := p.client.BlockByNumber(ctx, big.NewInt(int64(blockNumber)))
	if err != nil {
		err = fmt.Errorf("failed to get block %d: %v", blockNumber, err)
		return nil, err
	}

	if block == nil {
		logx.Infof("📦 [%s] Block %d is nil, returning empty transactions", p.chain.String(), blockNumber)
		return []*pb.Transaction{}, nil
	}

	blockHash := block.Hash().Hex()
	logx.Infof("📦 [%s] Block %d contains %d transactions (hash: %s)",
		p.chain.String(), blockNumber, len(block.Transactions()), blockHash)

	skipReceipts := provider.ShouldSkipReceipts(ctx)

	transactions := make([]*pb.Transaction, 0, len(block.Transactions()))
	for _, tx := range block.Transactions() {
		pbTx := p.convertEthTransactionToPbTransaction(tx, block.Number().Uint64(), int64(block.Time()), blockHash)

		if !skipReceipts {
			// Receipt is optional; keep tx pending if receipt fetch fails.
			txHash := tx.Hash()
			receipt, receiptErr := p.client.TransactionReceipt(ctx, txHash)
			if receiptErr == nil && receipt != nil {
				if receipt.Status == 1 {
					pbTx.Status = pb.TransactionStatus_TRANSACTION_STATUS_CONFIRMED
				} else {
					pbTx.Status = pb.TransactionStatus_TRANSACTION_STATUS_FAILED
				}

				pbTx.GasUsed = receipt.GasUsed

				if receipt.EffectiveGasPrice != nil && receipt.EffectiveGasPrice.Sign() > 0 {
					gasFeeWei := new(big.Int).Mul(receipt.EffectiveGasPrice, new(big.Int).SetUint64(receipt.GasUsed))
					pbTx.GasFee = units.FormatTokenAmount(gasFeeWei.String(), 18)
				}
			} else {
				logx.Debugf("⚠️ Failed to get receipt for tx %s: %v", txHash.Hex(), receiptErr)
			}
		}

		transactions = append(transactions, pbTx)
	}

	return transactions, nil
}
