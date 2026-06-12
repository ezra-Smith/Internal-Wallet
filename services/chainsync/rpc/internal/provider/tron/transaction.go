package tron

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
)

func (t *TronProvider) GetTransaction(ctx context.Context, txHash string) (*pb.Transaction, error) {
	start := time.Now()
	var err error
	defer t.finishRequest(start, &err)

	grpcClient := t.createClient()
	defer grpcClient.Stop()

	if err := t.connectWithTimeout(ctx, grpcClient); err != nil {
		err = fmt.Errorf("connection failed: %w", err)
		return nil, err
	}

	tx, err := grpcClient.GetTransactionByID(txHash)
	if err != nil {
		err = fmt.Errorf("get transaction by id failed: %w", err)
		return nil, err
	}
	if tx == nil || tx.RawData == nil {
		err = fmt.Errorf("invalid transaction response")
		return nil, err
	}

	transaction := &pb.Transaction{
		TxHash:         txHash,
		Chain:          pb.BlockChainType_CHAIN_TYPE_TRON,
		BlockTimestamp: tx.RawData.Timestamp,
		Status:         pb.TransactionStatus_TRANSACTION_STATUS_PENDING,
		// IMPORTANT: default to "0" instead of empty string, so downstream doesn't store NULL fee
		// when txInfo lookup fails (which happens on some nodes / pruning windows).
		GasFee: "0",
	}

	if len(tx.RawData.Contract) > 0 {
		contract := tx.RawData.Contract[0]
		if contract.Parameter != nil {
			transaction.Value = "0"
			transaction.InputData = contract.Parameter.Value
		}
	}

	if len(tx.Ret) > 0 {
		if tx.Ret[0].Ret == 0 {
			transaction.Status = pb.TransactionStatus_TRANSACTION_STATUS_CONFIRMED
		} else {
			transaction.Status = pb.TransactionStatus_TRANSACTION_STATUS_FAILED
		}
	}

	// Best-effort logs enrichment (optional; failure should not break ingestion).
	enriched, enrichErr := t.GetTransactionWithLogs(ctx, transaction)
	if enrichErr != nil {
		logx.Debugf("GetTransactionWithLogs failed for %s: %v", txHash, enrichErr)
		return transaction, nil
	}
	return enriched, nil
}

func (t *TronProvider) GetAddressBalance(ctx context.Context, address string, tokens []string) (*pb.GetAddressBalanceResp, error) {
	start := time.Now()
	var err error
	defer t.finishRequest(start, &err)

	grpcClient := t.createClient()
	defer grpcClient.Stop()

	if err := t.connectWithTimeout(ctx, grpcClient); err != nil {
		err = fmt.Errorf("connection failed: %w", err)
		return nil, err
	}

	account, err := grpcClient.GetAccount(address)
	if err != nil {
		// TRON 账户未激活时会返回错误，这是正常情况，返回零余额
		account = nil
	}

	var balance int64
	if account != nil && account.Balance > 0 {
		balance = account.Balance
	}

	// 转换为 TRX 格式（除以 10^6）
	trxBalance := float64(balance) / 1e6
	formattedBalance := fmt.Sprintf("%.6f", trxBalance)

	resp := &pb.GetAddressBalanceResp{
		Success:                true,
		NativeBalance:          strconv.FormatInt(balance, 10),
		FormattedNativeBalance: formattedBalance,
		TokenBalances:          []*pb.TokenBalance{},
		LastUpdated:            time.Now().Unix(),
	}

	// TODO: 查询代币余额
	_ = tokens

	return resp, nil
}

func (t *TronProvider) GetAddressTransactions(ctx context.Context, address string, startBlock, endBlock uint64, page, pageSize int32) (*pb.GetAddressTransactionsResp, error) {
	start := time.Now()
	var err error
	defer t.finishRequest(start, &err)

	_ = address
	_ = endBlock
	_ = page
	_ = pageSize

	logx.Infof("📝 TRON address transaction query not directly supported, use block scanning instead")
	return &pb.GetAddressTransactionsResp{
		Success:      true,
		Transactions: []*pb.Transaction{},
		Total:        0,
		CurrentBlock: startBlock,
	}, nil
}
