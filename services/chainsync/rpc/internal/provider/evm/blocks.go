package evm

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"time"

	"internalwallet/proto/pb"
)

func (p *Web3Provider) GetLatestBlock(ctx context.Context) (uint64, error) {
	start := time.Now()
	var err error
	defer p.finishRequest(start, &err)

	latestBlock, err := p.client.BlockByNumber(ctx, nil)
	if err != nil {
		err = fmt.Errorf("failed to get latest block: %v", err)
		return 0, err
	}

	return latestBlock.Number().Uint64(), nil
}

func (p *Web3Provider) GetBlock(ctx context.Context, blockNumber uint64) (*pb.ChainBlockInfo, error) {
	start := time.Now()
	var err error
	defer p.finishRequest(start, &err)

	block, err := p.client.BlockByNumber(ctx, big.NewInt(int64(blockNumber)))
	if err != nil {
		err = fmt.Errorf("failed to get block %d: %v", blockNumber, err)
		return nil, err
	}

	return &pb.ChainBlockInfo{
		BlockNumber:      block.Number().Uint64(),
		BlockHash:        block.Hash().Hex(),
		ParentHash:       block.ParentHash().Hex(),
		Timestamp:        int64(block.Time()),
		TransactionCount: uint64(len(block.Transactions())),
		Miner:            block.Coinbase().String(),
		Difficulty:       block.Difficulty().String(),
		Size:             strconv.FormatUint(block.Size(), 10),
		GasLimit:         block.GasLimit(),
		GasUsed:          block.GasUsed(),
		ChainId:          p.chainID.Int64(),
	}, nil
}
