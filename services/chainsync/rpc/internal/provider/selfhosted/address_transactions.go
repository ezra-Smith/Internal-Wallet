package selfhosted

import (
	"context"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
)

func (p *SelfHostedProvider) GetAddressTransactions(ctx context.Context, address string, startBlock, endBlock uint64, page, pageSize int32) (txResp *pb.GetAddressTransactionsResp, err error) {
	start := time.Now()
	defer p.finishRequest(start, &err)

	logx.Infof("🔍 Getting transactions for address: %s, chain: %v, page: %d, size: %d", address, p.config.Chain, page, pageSize)

	if address == "" {
		return nil, fmt.Errorf("address cannot be empty")
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 50
	}

	if endBlock == 0 {
		latestBlock, err := p.GetLatestBlock(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get latest block: %v", err)
		}
		endBlock = latestBlock
	}

	if startBlock == 0 {
		if endBlock > 10 {
			startBlock = endBlock - 10
		} else {
			startBlock = 0
		}
	}

	logx.Infof("🔍 Scanning blocks %d to %d for address: %s", startBlock, endBlock, address)

	transactions := make([]*pb.Transaction, 0)

	for i := endBlock; i >= startBlock && len(transactions) < int(pageSize); i-- {
		block, err := p.GetBlock(ctx, i)
		if err != nil {
			logx.Errorf("Failed to get block %d: %v", i, err)
		} else {
			blockTxs := p.scanBlockForAddress(block, address)
			transactions = append(transactions, blockTxs...)
		}

		if i == startBlock {
			break
		}

		time.Sleep(100 * time.Millisecond)
	}

	if len(transactions) > int(pageSize) {
		transactions = transactions[:pageSize]
	}

	logx.Infof("Found %d transactions for address %s in blocks %d to %d", len(transactions), address, startBlock, endBlock)

	return &pb.GetAddressTransactionsResp{
		Success:      true,
		Transactions: transactions,
		Total:        int32(len(transactions)),
	}, nil
}
