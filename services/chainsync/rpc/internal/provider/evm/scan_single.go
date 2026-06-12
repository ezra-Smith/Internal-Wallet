package evm

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
)

func (p *Web3Provider) singleScanAddressTransactions(ctx context.Context, address common.Address, startBlock, endBlock uint64, pageSize int) ([]*pb.Transaction, error) {
	logx.Infof("📝 Using single scan mode for blocks %d to %d", startBlock, endBlock)

	transactions := make([]*pb.Transaction, 0)

	for num := endBlock; num >= startBlock && len(transactions) < pageSize; num-- {
		block, err := p.getBlockWithRetry(ctx, num)
		if err != nil {
			logx.Errorf("Failed to get block %d: %v", num, err)
		} else {
			blockTxs := p.scanBlockForAddress(block, address)
			transactions = append(transactions, blockTxs...)
		}

		if num == startBlock {
			break
		}

		time.Sleep(200 * time.Millisecond)
	}

	if len(transactions) > pageSize {
		transactions = transactions[:pageSize]
	}

	return transactions, nil
}

func (p *Web3Provider) scanBlockForAddress(block *types.Block, address common.Address) []*pb.Transaction {
	transactions := make([]*pb.Transaction, 0)

	for _, tx := range block.Transactions() {
		chainID := p.chainID
		if chainID == nil || chainID.Sign() <= 0 {
			chainID = big.NewInt(1)
		}

		signer := types.NewEIP155Signer(chainID)
		from, err := types.Sender(signer, tx)
		if err != nil {
			logx.Errorf("Failed to get transaction sender: %v", err)
			continue
		}

		isFrom := strings.EqualFold(from.Hex(), address.Hex())
		isTo := tx.To() != nil && strings.EqualFold(tx.To().Hex(), address.Hex())

		if !isFrom && !isTo {
			continue
		}

		txIndex := 0
		for i, blockTx := range block.Transactions() {
			if blockTx.Hash() == tx.Hash() {
				txIndex = i
				break
			}
		}

		pbTx := &pb.Transaction{
			TxHash:           tx.Hash().Hex(),
			Chain:            p.chain,
			BlockNumber:      block.Number().Uint64(),
			BlockHash:        block.Hash().Hex(),
			TransactionIndex: uint64(txIndex),
			FromAddress:      from.String(),
			ToAddress:        "",
			Value:            tx.Value().String(),
			GasPrice:         tx.GasPrice().String(),
			GasUsed:          0,
			Status:           pb.TransactionStatus_TRANSACTION_STATUS_PENDING,
			Nonce:            fmt.Sprintf("%d", tx.Nonce()),
			BlockTimestamp:   int64(block.Time()),
		}

		if tx.To() != nil {
			pbTx.ToAddress = tx.To().String()
		}

		transactions = append(transactions, pbTx)
		logx.Infof("Found transaction: %s from %s to %s, value: %s",
			pbTx.TxHash,
			pbTx.FromAddress,
			func() string {
				if pbTx.ToAddress != "" {
					return pbTx.ToAddress
				}
				return "Contract Creation"
			}(),
			pbTx.Value,
		)
	}

	return transactions
}

