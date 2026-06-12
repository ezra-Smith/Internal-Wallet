package tron

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/fbsobreira/gotron-sdk/pkg/common"
	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
)

func (t *TronProvider) GetLatestBlock(ctx context.Context) (uint64, error) {
	start := time.Now()
	var err error
	defer t.finishRequest(start, &err)

	var lastErr error
	maxRetries := 3

	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			logx.Infof("TRON GetLatestBlock retry %d/%d", i, maxRetries-1)
			time.Sleep(time.Duration(i) * time.Second)
		}

		grpcClient := t.createClient()
		if err := t.connectWithTimeout(ctx, grpcClient); err != nil {
			grpcClient.Stop()
			lastErr = fmt.Errorf("connection failed: %w", err)
			continue
		}

		block, err := grpcClient.GetNowBlock()
		grpcClient.Stop()

		if err != nil {
			lastErr = fmt.Errorf("get latest block failed: %w", err)
			continue
		}

		if block == nil || block.BlockHeader == nil || block.BlockHeader.RawData == nil {
			lastErr = fmt.Errorf("invalid block response")
			continue
		}

		return uint64(block.BlockHeader.RawData.Number), nil
	}

	err = lastErr
	return 0, err
}

func (t *TronProvider) GetBlock(ctx context.Context, blockNumber uint64) (*pb.ChainBlockInfo, error) {
	start := time.Now()
	var err error
	defer t.finishRequest(start, &err)

	grpcClient := t.createClient()
	defer grpcClient.Stop()

	if err := t.connectWithTimeout(ctx, grpcClient); err != nil {
		err = fmt.Errorf("connection failed: %w", err)
		return nil, err
	}

	block, err := grpcClient.GetBlockByNum(int64(blockNumber))
	if err != nil {
		err = fmt.Errorf("get block by number failed: %w", err)
		return nil, err
	}

	if block == nil || block.BlockHeader == nil || block.BlockHeader.RawData == nil {
		err = fmt.Errorf("invalid block response")
		return nil, err
	}

	parentHash := common.Bytes2Hex(block.BlockHeader.RawData.ParentHash)

	var blockHash string
	if len(block.Blockid) == 32 {
		blockHash = hex.EncodeToString(block.Blockid)
		logx.Debugf("✅ Got TRON block ID from API for block %d: %s", blockNumber, blockHash)
	} else {
		blockHash = t.calculateTronBlockIDFromHeader(block.BlockHeader)
		logx.Debugf("⚠️ Calculated TRON block ID for block %d: %s", blockNumber, blockHash)
	}

	return &pb.ChainBlockInfo{
		BlockNumber:      uint64(block.BlockHeader.RawData.Number),
		BlockHash:        blockHash,
		ParentHash:       parentHash,
		Timestamp:        block.BlockHeader.RawData.Timestamp,
		TransactionCount: uint64(len(block.Transactions)),
		Miner:            common.Bytes2Hex(block.BlockHeader.RawData.WitnessAddress),
		ChainId:          t.chainID.Int64(),
	}, nil
}
