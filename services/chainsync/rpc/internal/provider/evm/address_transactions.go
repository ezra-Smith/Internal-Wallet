package evm

import (
	"context"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
)

func (p *Web3Provider) GetAddressTransactions(ctx context.Context, address string, startBlock, endBlock uint64, page, pageSize int32) (*pb.GetAddressTransactionsResp, error) {
	start := time.Now()
	var err error
	defer p.finishRequest(start, &err)

	if p.client == nil {
		err = fmt.Errorf("ethereum client is nil")
		return nil, err
	}

	logx.Infof("🔍 Getting transactions for address: %s, chain: %v, page: %d, size: %d", address, p.chain, page, pageSize)

	if address == "" {
		err = fmt.Errorf("address cannot be empty")
		return nil, err
	}

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 50
	}

	addr := common.HexToAddress(address)

	if endBlock == 0 {
		header, err := p.client.HeaderByNumber(ctx, nil)
		if err != nil {
			err = fmt.Errorf("failed to get latest block header: %v", err)
			return nil, err
		}
		endBlock = header.Number.Uint64()
	}

	if startBlock == 0 {
		if endBlock > 20 {
			startBlock = endBlock - 20
		} else {
			startBlock = 0
		}
	}

	logx.Infof("🔍 Scanning blocks %d to %d for address: %s", startBlock, endBlock, address)

	batchConfig := BatchConfig{
		Enabled:          true,
		MaxBatchSize:     10,
		MaxConcurrency:   1,
		BatchTimeout:     10 * time.Second,
		RetryAttempts:    3,
		ProgressInterval: 10 * time.Second,
	}

	var transactions []*pb.Transaction

	totalBlocks := endBlock - startBlock + 1
	if batchConfig.Enabled && totalBlocks > uint64(batchConfig.MaxBatchSize) {
		logx.Infof("🚀 Using batch scan mode for large range: %d blocks", totalBlocks)
		transactions, err = p.batchScanAddressTransactions(ctx, addr, startBlock, endBlock, int(pageSize), batchConfig)
	} else {
		logx.Infof("📝 Using single scan mode for small range: %d blocks", totalBlocks)
		transactions, err = p.singleScanAddressTransactions(ctx, addr, startBlock, endBlock, int(pageSize))
	}

	if err != nil {
		err = fmt.Errorf("failed to scan transactions: %v", err)
		return nil, err
	}

	total := int32(len(transactions))
	logx.Infof("Found %d transactions for address %s in blocks %d to %d", len(transactions), address, startBlock, endBlock)

	return &pb.GetAddressTransactionsResp{
		Success:      true,
		Transactions: transactions,
		Total:        total,
	}, nil
}
