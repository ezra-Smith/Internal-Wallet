package selfhosted

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"internalwallet/proto/pb"
)

func (p *SelfHostedProvider) GetLatestBlock(ctx context.Context) (block uint64, err error) {
	start := time.Now()
	defer p.finishRequest(start, &err)

	if p.config.Chain == pb.BlockChainType_CHAIN_TYPE_TRON {
		resp, err := p.getTronNowBlock(ctx)
		if err != nil {
			return 0, fmt.Errorf("failed to get TRON latest block: %v", err)
		}
		return uint64(resp.BlockHeader.RawData.Number), nil
	}

	resp, err := p.callRPC(ctx, "eth_blockNumber", []interface{}{})
	if err != nil {
		return 0, fmt.Errorf("failed to get latest block: %v", err)
	}

	var blockNumberHex string
	if err := json.Unmarshal(resp.Result, &blockNumberHex); err != nil {
		return 0, fmt.Errorf("failed to unmarshal block number: %v", err)
	}

	blockNumber, err := strconv.ParseInt(blockNumberHex[2:], 16, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse block number: %v", err)
	}

	return uint64(blockNumber), nil
}

func (p *SelfHostedProvider) GetBlock(ctx context.Context, blockNumber uint64) (blockInfo *pb.ChainBlockInfo, err error) {
	start := time.Now()
	defer p.finishRequest(start, &err)

	if p.config.Chain == pb.BlockChainType_CHAIN_TYPE_TRON {
		resp, err := p.getTronBlockByNumber(ctx, blockNumber)
		if err != nil {
			return nil, fmt.Errorf("failed to get TRON block %d: %v", blockNumber, err)
		}

		txCount := len(resp.Transactions)
		return &pb.ChainBlockInfo{
			BlockNumber:      uint64(resp.BlockHeader.RawData.Number),
			BlockHash:        resp.BlockID,
			ParentHash:       resp.BlockHeader.RawData.ParentHash,
			Timestamp:        resp.BlockHeader.RawData.Timestamp,
			TransactionCount: uint64(txCount),
			Miner:            resp.BlockHeader.RawData.WitnessAddress,
			ChainId:          p.chainID,
		}, nil
	}

	blockNumberHex := fmt.Sprintf("0x%x", blockNumber)
	resp, err := p.callRPC(ctx, "eth_getBlockByNumber", []interface{}{blockNumberHex, true})
	if err != nil {
		return nil, fmt.Errorf("failed to get block %d: %v", blockNumber, err)
	}

	var block map[string]interface{}
	if err := json.Unmarshal(resp.Result, &block); err != nil {
		return nil, fmt.Errorf("failed to unmarshal block: %v", err)
	}

	result := &pb.ChainBlockInfo{
		BlockNumber: blockNumber,
		ChainId:     p.chainID,
	}

	if hash, ok := block["hash"].(string); ok {
		result.BlockHash = hash
	}
	if parentHash, ok := block["parentHash"].(string); ok {
		result.ParentHash = parentHash
	}
	if timestamp, ok := block["timestamp"].(string); ok {
		if ts, err := strconv.ParseInt(timestamp[2:], 16, 64); err == nil {
			result.Timestamp = ts * 1000
		}
	}
	if miner, ok := block["miner"].(string); ok {
		result.Miner = miner
	}
	if difficulty, ok := block["difficulty"].(string); ok {
		result.Difficulty = difficulty
	}
	if gasLimit, ok := block["gasLimit"].(string); ok {
		if gl, err := strconv.ParseInt(gasLimit[2:], 16, 64); err == nil {
			result.GasLimit = uint64(gl)
		}
	}
	if gasUsed, ok := block["gasUsed"].(string); ok {
		if gu, err := strconv.ParseInt(gasUsed[2:], 16, 64); err == nil {
			result.GasUsed = uint64(gu)
		}
	}
	if size, ok := block["size"].(string); ok {
		result.Size = size
	}
	if transactions, ok := block["transactions"].([]interface{}); ok {
		result.TransactionCount = uint64(len(transactions))
	}

	return result, nil
}
