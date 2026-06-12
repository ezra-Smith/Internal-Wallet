package selfhosted

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
)

func (p *SelfHostedProvider) getTronBlockTransactions(ctx context.Context, blockNumber uint64) ([]*pb.Transaction, error) {
	blockResp, err := p.getTronBlockByNumber(ctx, blockNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get TRON block %d: %v", blockNumber, err)
	}

	if blockResp == nil || len(blockResp.Transactions) == 0 {
		logx.Infof("📦 TRON block %d has no transactions", blockNumber)
		return []*pb.Transaction{}, nil
	}

	block := &pb.ChainBlockInfo{
		BlockNumber: blockNumber,
		BlockHash:   blockResp.BlockID,
		Timestamp:   blockResp.BlockHeader.RawData.Timestamp,
	}

	logx.Infof("📦 Processing %d transactions from TRON block %d", len(blockResp.Transactions), blockNumber)

	transactions := make([]*pb.Transaction, 0, len(blockResp.Transactions))
	for i, tronTx := range blockResp.Transactions {
		pbTx := p.parseTronTransaction(&tronTx, block, uint32(i))
		if pbTx == nil {
			continue
		}

		transactions = append(transactions, pbTx)
	}

	logx.Infof("✅ Successfully parsed %d transactions from TRON block %d", len(transactions), blockNumber)
	return transactions, nil
}
