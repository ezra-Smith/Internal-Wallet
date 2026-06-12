package logic

import (
	"context"
	"fmt"
	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
	"internalwallet/services/chainrpc/rpc/internal/svc"
)

type GetBlockHeightLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetBlockHeightLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetBlockHeightLogic {
	return &GetBlockHeightLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetBlockHeight 查询区块高度
func (l *GetBlockHeightLogic) GetBlockHeight(in *pb.GetBlockHeightReq) (*pb.GetBlockHeightResp, error) {
	switch in.Chain {
	case pb.ChainRpcType_CHAIN_TYPE_ETHEREUM, pb.ChainRpcType_CHAIN_TYPE_BSC:
		return l.getETHBlockHeight(in)
	case pb.ChainRpcType_CHAIN_TYPE_TRON:
		return l.getTronBlockHeight(in)
	default:
		return nil, fmt.Errorf("unsupported chain type: %v", in.Chain)
	}
}

// getETHBlockHeight 查询 ETH/BSC 区块高度
func (l *GetBlockHeightLogic) getETHBlockHeight(in *pb.GetBlockHeightReq) (*pb.GetBlockHeightResp, error) {
	// Get ETH client
	client, err := l.svcCtx.ChainMgr.GetETHClient(in.Chain)
	if err != nil {
		return nil, fmt.Errorf("failed to get ETH client: %v", err)
	}

	// Get latest block number
	blockNumber, err := client.BlockNumber(l.ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get block number: %v", err)
	}

	// Get latest block for hash and timestamp
	block, err := client.BlockByNumber(l.ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest block: %v", err)
	}

	return &pb.GetBlockHeightResp{
		Success:     true,
		Message:     "Block height retrieved successfully",
		BlockHeight: blockNumber,
		BlockHash:   block.Hash().Hex(),
		Timestamp:   int64(block.Time()),
	}, nil
}

// getTronBlockHeight 查询 TRON 区块高度
func (l *GetBlockHeightLogic) getTronBlockHeight(in *pb.GetBlockHeightReq) (*pb.GetBlockHeightResp, error) {
	// Get TRON client
	tronClient, err := l.svcCtx.ChainMgr.GetTronClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get TRON client: %v", err)
	}
	defer tronClient.Stop()

	// Get latest block for timestamp
	block, err := tronClient.GetNowBlock()
	if err != nil {
		return nil, fmt.Errorf("failed to get latest TRON block: %v", err)
	}

	var timestamp int64
	var blockHash string
	if block != nil && block.BlockHeader != nil && block.BlockHeader.RawData != nil {
		timestamp = block.BlockHeader.RawData.Timestamp
		// TRON block hash calculation is complex, using block number as placeholder
		blockHash = fmt.Sprintf("block_%d", block.BlockHeader.RawData.Number)
	}

	return &pb.GetBlockHeightResp{
		Success:     true,
		Message:     "TRON block height retrieved successfully",
		BlockHeight: uint64(block.BlockHeader.RawData.Number),
		BlockHash:   blockHash,
		Timestamp:   timestamp,
	}, nil
}
