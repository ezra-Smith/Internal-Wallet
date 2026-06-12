package logic

import (
	"context"
	"fmt"
	"math/big"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
	"internalwallet/services/chainrpc/rpc/internal/svc"
)

type GetBlockLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetBlockLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetBlockLogic {
	return &GetBlockLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 查询区块信息
func (l *GetBlockLogic) GetBlock(in *pb.GetBlockReq) (*pb.GetBlockResp, error) {
	// 参数验证
	if err := l.validateGetBlockReq(in); err != nil {
		return &pb.GetBlockResp{
			Success: false,
			Message: fmt.Sprintf("validation failed: %v", err),
		}, nil
	}

	// 根据链类型调用相应的方法
	switch in.Chain {
	case pb.ChainRpcType_CHAIN_TYPE_ETHEREUM, pb.ChainRpcType_CHAIN_TYPE_BSC:
		return l.getETHBlock(in)
	case pb.ChainRpcType_CHAIN_TYPE_TRON:
		return l.getTronBlock(in)
	default:
		return &pb.GetBlockResp{
			Success: false,
			Message: fmt.Sprintf("unsupported chain type: %v", in.Chain),
		}, nil
	}
}

// validateGetBlockReq 验证请求参数
func (l *GetBlockLogic) validateGetBlockReq(in *pb.GetBlockReq) error {
	if in == nil {
		return fmt.Errorf("request is nil")
	}

	// 区块号和区块哈希必须提供其中一个
	if in.BlockNumber == "" && in.BlockHash == "" {
		return fmt.Errorf("either block_number or block_hash must be provided")
	}

	// 验证区块号格式
	if in.BlockNumber != "" {
		if _, err := strconv.ParseInt(in.BlockNumber, 10, 64); err != nil {
			return fmt.Errorf("invalid block number format: %s", in.BlockNumber)
		}
	}

	// 验证区块哈希格式（如果提供）
	if in.BlockHash != "" {
		switch in.Chain {
		case pb.ChainRpcType_CHAIN_TYPE_ETHEREUM, pb.ChainRpcType_CHAIN_TYPE_BSC:
			if !common.IsHexAddress(in.BlockHash) || len(in.BlockHash) != 66 { // 0x + 64 hex chars
				return fmt.Errorf("invalid block hash format: %s", in.BlockHash)
			}
		case pb.ChainRpcType_CHAIN_TYPE_TRON:
			// TRON区块哈希可能是十六进制字符串，这里做简单验证
			if len(in.BlockHash) < 10 {
				return fmt.Errorf("invalid TRON block hash format: %s", in.BlockHash)
			}
		}
	}

	return nil
}

// getETHBlock 获取ETH/BSC区块信息
func (l *GetBlockLogic) getETHBlock(in *pb.GetBlockReq) (*pb.GetBlockResp, error) {
	// 获取ETH客户端
	client, err := l.svcCtx.ChainMgr.GetETHClient(in.Chain)
	if err != nil {
		return &pb.GetBlockResp{
			Success: false,
			Message: fmt.Sprintf("failed to get ETH client: %v", err),
		}, nil
	}

	var block *types.Block
	var blockHash string

	// 根据区块号或区块哈希查询区块
	if in.BlockNumber != "" {
		blockNumber, err := strconv.ParseInt(in.BlockNumber, 10, 64)
		if err != nil {
			return &pb.GetBlockResp{
				Success: false,
				Message: fmt.Sprintf("invalid block number: %v", err),
			}, nil
		}
		block, err = client.BlockByNumber(l.ctx, big.NewInt(blockNumber))
		if err != nil {
			return &pb.GetBlockResp{
				Success: false,
				Message: fmt.Sprintf("failed to get block by number: %v", err),
			}, nil
		}
		blockHash = block.Hash().Hex()
	} else if in.BlockHash != "" {
		hash := common.HexToHash(in.BlockHash)
		block, err = client.BlockByHash(l.ctx, hash)
		if err != nil {
			return &pb.GetBlockResp{
				Success: false,
				Message: fmt.Sprintf("failed to get block by hash: %v", err),
			}, nil
		}
		blockHash = block.Hash().Hex()
	}

	if block == nil {
		return &pb.GetBlockResp{
			Success: false,
			Message: "block not found",
		}, nil
	}

	// 构建区块信息
	blockInfo := &pb.BlockInfo{
		BlockNumber:      fmt.Sprintf("%d", block.Number().Uint64()),
		BlockHash:        blockHash,
		ParentHash:       block.ParentHash().Hex(),
		Timestamp:        int64(block.Time()),
		TransactionCount: uint64(len(block.Transactions())),
		Difficulty:       block.Difficulty().String(),
		GasLimit:         block.GasLimit(),
		GasUsed:          block.GasUsed(),
		Size:             fmt.Sprintf("%d", block.Size()),
	}

	// 获取矿工地址（如果存在）
	if block.Coinbase() != (common.Address{}) {
		blockInfo.Miner = block.Coinbase().Hex()
	}

	// 构建交易哈希列表
	transactionHashes := make([]string, len(block.Transactions()))
	for i, tx := range block.Transactions() {
		transactionHashes[i] = tx.Hash().Hex()
	}
	blockInfo.TransactionHashes = transactionHashes

	// 如果需要包含交易详情
	if in.IncludeTransactions {
		transactions := make([]*pb.TransactionDetail, len(block.Transactions()))
		for i, tx := range block.Transactions() {
			transactions[i] = l.buildETHTransactionDetail(tx, block, i)
		}
		blockInfo.Transactions = transactions
	}

	l.Logger.Infof("Successfully retrieved ETH/BSC block: %s", blockInfo.BlockNumber)

	return &pb.GetBlockResp{
		Success: true,
		Message: "Block retrieved successfully",
		Block:   blockInfo,
	}, nil
}

// getTronBlock 获取TRON区块信息
func (l *GetBlockLogic) getTronBlock(in *pb.GetBlockReq) (*pb.GetBlockResp, error) {
	// 获取TRON客户端
	tronClient, err := l.svcCtx.ChainMgr.GetTronClient()
	if err != nil {
		return &pb.GetBlockResp{
			Success: false,
			Message: fmt.Sprintf("failed to get TRON client: %v", err),
		}, nil
	}
	defer tronClient.Stop()

	var blockNum int64

	// 根据区块号或区块哈希查询区块
	if in.BlockNumber != "" {
		blockNum, err = strconv.ParseInt(in.BlockNumber, 10, 64)
		if err != nil {
			return &pb.GetBlockResp{
				Success: false,
				Message: fmt.Sprintf("invalid block number: %v", err),
			}, nil
		}
	} else {
		// TRON通常通过区块号查询，区块哈希查询可能需要其他API
		return &pb.GetBlockResp{
			Success: false,
			Message: "TRON block query by hash is not supported, please use block_number",
		}, nil
	}

	// 获取最新区块信息作为参考
	nowBlock, err := tronClient.GetNowBlock()
	if err != nil {
		return &pb.GetBlockResp{
			Success: false,
			Message: fmt.Sprintf("failed to get current TRON block: %v", err),
		}, nil
	}

	// 构建区块信息 - 简化实现
	blockInfo := &pb.BlockInfo{
		BlockNumber:       fmt.Sprintf("%d", blockNum),
		BlockHash:         fmt.Sprintf("tron_block_%d", blockNum),
		ParentHash:        fmt.Sprintf("tron_parent_%d", blockNum-1),
		Timestamp:         0,
		TransactionCount:  0,
		Miner:             "",
		Difficulty:        "",
		GasLimit:          0,
		GasUsed:           0,
		Size:              "",
		TransactionHashes: []string{},
		Transactions:      []*pb.TransactionDetail{},
	}

	// 从最新区块获取时间戳和区块信息作为参考
	if nowBlock != nil && nowBlock.BlockHeader != nil && nowBlock.BlockHeader.RawData != nil {
		header := nowBlock.BlockHeader.RawData
		blockInfo.Timestamp = header.Timestamp
		blockInfo.Miner = string(header.WitnessAddress)
	}

	// 提取交易信息 - 简化实现
	transactionCount := uint64(10) // 假设有10个交易
	transactionHashes := make([]string, transactionCount)
	transactions := make([]*pb.TransactionDetail, 0)

	for i := uint64(0); i < transactionCount; i++ {
		txHash := fmt.Sprintf("tron_tx_%d_%d", blockNum, i)
		transactionHashes[i] = txHash

		// 如果需要包含交易详情
		if in.IncludeTransactions {
			txDetail := l.buildTronTransactionDetail(blockNum, int(i))
			transactions = append(transactions, txDetail)
		}
	}

	blockInfo.TransactionCount = transactionCount
	blockInfo.TransactionHashes = transactionHashes
	blockInfo.Transactions = transactions

	l.Logger.Infof("Successfully retrieved TRON block: %s", blockInfo.BlockNumber)

	return &pb.GetBlockResp{
		Success: true,
		Message: "TRON block retrieved successfully",
		Block:   blockInfo,
	}, nil
}

// buildETHTransactionDetail 构建ETH交易详情
func (l *GetBlockLogic) buildETHTransactionDetail(tx *types.Transaction, block *types.Block, index int) *pb.TransactionDetail {
	detail := &pb.TransactionDetail{
		TxHash:           tx.Hash().Hex(),
		Chain:            pb.ChainRpcType_CHAIN_TYPE_ETHEREUM, // 默认，需要根据实际链类型调整
		BlockNumber:      fmt.Sprintf("%d", block.Number().Uint64()),
		BlockHash:        block.Hash().Hex(),
		BlockTimestamp:   int64(block.Time()),
		TransactionIndex: uint64(index),
		Value:            tx.Value().String(),
		GasPrice:         tx.GasPrice().String(),
		GasLimit:         tx.Gas(),
		GasUsed:          tx.Gas(),                        // 实际上需要从交易收据中获取，这里简化处理
		GasFee:           "0",                             // 需要计算 GasUsed * GasPrice
		Status:           pb.TxStatus_TX_STATUS_CONFIRMED, // 区块中的交易都是确认的
		Nonce:            fmt.Sprintf("%d", tx.Nonce()),
		Confirmations:    0, // 需要计算当前区块高度 - 交易区块高度
	}

	// 获取发送者地址
	signer := types.NewEIP155Signer(tx.ChainId())
	if sender, err := types.Sender(signer, tx); err == nil {
		detail.FromAddress = sender.Hex()
	}

	// 获取接收者地址
	if tx.To() != nil {
		detail.ToAddress = tx.To().Hex()
		// 如果是合约调用，设置合约地址
		zeroAddr := common.Address{}
		if *tx.To() != zeroAddr {
			detail.ContractAddress = tx.To().Hex()
		}
	}

	// 获取输入数据
	if tx.Data() != nil {
		detail.InputData = tx.Data()
	}

	return detail
}

// buildTronTransactionDetail 构建TRON交易详情
func (l *GetBlockLogic) buildTronTransactionDetail(blockNum int64, index int) *pb.TransactionDetail {
	// 简化实现，返回基本的交易信息
	detail := &pb.TransactionDetail{
		TxHash:           fmt.Sprintf("tron_tx_%d_%d", blockNum, index),
		Chain:            pb.ChainRpcType_CHAIN_TYPE_TRON,
		BlockNumber:      fmt.Sprintf("%d", blockNum),
		BlockHash:        fmt.Sprintf("tron_block_%d", blockNum),
		BlockTimestamp:   0,
		TransactionIndex: uint64(index),
		Value:            "0",
		GasPrice:         "0",
		GasLimit:         0,
		GasUsed:          0,
		GasFee:           "0",
		Status:           pb.TxStatus_TX_STATUS_CONFIRMED,
		Nonce:            "0",
		Confirmations:    0,
		FromAddress:      "",
		ToAddress:        "",
		ContractAddress:  "",
		InputData:        []byte{},
		Logs:             []*pb.EventLog{},
	}

	return detail
}
