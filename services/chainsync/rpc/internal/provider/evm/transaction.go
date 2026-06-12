package evm

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"

	"internalwallet/proto/pb"
	"internalwallet/services/chainsync/rpc/internal/units"
)

func (p *Web3Provider) GetTransaction(ctx context.Context, txHash string) (transaction *pb.Transaction, err error) {
	start := time.Now()
	defer p.finishRequest(start, &err)

	tx, pending, err := p.client.TransactionByHash(ctx, common.HexToHash(txHash))
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction %s: %v", txHash, err)
	}

	if pending {
		return nil, fmt.Errorf("transaction %s is still pending", txHash)
	}

	receipt, err := p.client.TransactionReceipt(ctx, common.HexToHash(txHash))
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction receipt %s: %v", txHash, err)
	}

	block, err := p.client.BlockByHash(ctx, receipt.BlockHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get block for transaction %s: %v", txHash, err)
	}

	// from address: use signer based on tx chain id, with fallback.
	signer := types.LatestSignerForChainID(tx.ChainId())
	from, err := types.Sender(signer, tx)
	if err != nil {
		fromAddr := p.recoverSenderAddress(tx)
		if fromAddr == "" {
			return nil, fmt.Errorf("failed to get sender address: %v", err)
		}
		from = common.HexToAddress(fromAddr)
	}

	var toAddress string
	if tx.To() != nil {
		toAddress = tx.To().String()
	}

	status := pb.TransactionStatus_TRANSACTION_STATUS_FAILED
	if receipt.Status == 1 {
		status = pb.TransactionStatus_TRANSACTION_STATUS_CONFIRMED
	}

	transactionLogs := make([]*pb.TransactionLog, 0, len(receipt.Logs))
	for _, receiptLog := range receipt.Logs {
		topics := make([]string, 0, len(receiptLog.Topics))
		for _, topic := range receiptLog.Topics {
			topics = append(topics, topic.Hex())
		}

		transactionLogs = append(transactionLogs, &pb.TransactionLog{
			LogIndex:        strconv.FormatUint(uint64(receiptLog.Index), 10),
			TransactionHash: receiptLog.TxHash.Hex(),
			Address:         receiptLog.Address.String(),
			Topics:          topics,
			Data:            receiptLog.Data,
			BlockNumber:     receiptLog.BlockNumber,
			BlockTimestamp:  int64(block.Time()),
		})
	}

	effectiveGasPrice := tx.GasPrice()
	if receipt.EffectiveGasPrice != nil && receipt.EffectiveGasPrice.Sign() > 0 {
		effectiveGasPrice = receipt.EffectiveGasPrice
	}

	formattedValue := units.FormatTokenAmount(tx.Value().String(), 18)
	formattedGasPrice := units.FormatTokenAmount(effectiveGasPrice.String(), 9)

	gasFeeWei := new(big.Int).Mul(effectiveGasPrice, new(big.Int).SetUint64(receipt.GasUsed))
	formattedGasFee := units.FormatTokenAmount(gasFeeWei.String(), 18)

	return &pb.Transaction{
		TxHash:           tx.Hash().Hex(),
		Chain:            p.chain,
		BlockNumber:      receipt.BlockNumber.Uint64(),
		BlockHash:        receipt.BlockHash.Hex(),
		BlockTimestamp:   int64(block.Time()),
		TransactionIndex: uint64(receipt.TransactionIndex),
		FromAddress:      from.String(),
		ToAddress:        toAddress,
		Value:            formattedValue,
		GasPrice:         formattedGasPrice,
		GasUsed:          receipt.GasUsed,
		GasFee:           formattedGasFee,
		Status:           status,
		Nonce:            strconv.FormatUint(tx.Nonce(), 10),
		InputData:        tx.Data(),
		Confirmations:    0,
		ContractAddress:  "",
		Logs:             transactionLogs,
	}, nil
}
