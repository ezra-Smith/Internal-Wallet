package logic

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
	"internalwallet/services/chainrpc/rpc/internal/svc"
)

type GetTransactionStatusLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetTransactionStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetTransactionStatusLogic {
	return &GetTransactionStatusLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetTransactionStatus 查询交易状态
func (l *GetTransactionStatusLogic) GetTransactionStatus(in *pb.GetTransactionStatusReq) (*pb.GetTransactionStatusResp, error) {
	switch in.Chain {
	case pb.ChainRpcType_CHAIN_TYPE_ETHEREUM, pb.ChainRpcType_CHAIN_TYPE_BSC:
		return l.getETHTransactionStatus(in)
	case pb.ChainRpcType_CHAIN_TYPE_TRON:
		return l.getTronTransactionStatus(in)
	default:
		return nil, fmt.Errorf("unsupported chain type: %v", in.Chain)
	}
}

// getETHTransactionStatus 查询 ETH/BSC 交易状态
func (l *GetTransactionStatusLogic) getETHTransactionStatus(in *pb.GetTransactionStatusReq) (*pb.GetTransactionStatusResp, error) {
	// Get ETH client
	client, err := l.svcCtx.ChainMgr.GetETHClient(in.Chain)
	if err != nil {
		return nil, fmt.Errorf("failed to get ETH client: %v", err)
	}

	// Parse transaction hash
	txHash := common.HexToHash(in.TxHash)

	// Get transaction
	tx, _, err := client.TransactionByHash(l.ctx, txHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction: %v", err)
	}

	// If transaction is pending
	if tx == nil {
		return &pb.GetTransactionStatusResp{
			Success:               true,
			Message:               "Transaction not found",
			Status:                pb.TxStatus_TX_STATUS_PENDING,
			Confirmations:         0,
			RequiredConfirmations: 3, // Default
			IsConfirmed:           false,
		}, nil
	}

	// Get transaction receipt
	receipt, err := client.TransactionReceipt(l.ctx, txHash)
	if err != nil {
		// Transaction is still pending
		return &pb.GetTransactionStatusResp{
			Success:               true,
			Message:               "Transaction pending",
			Status:                pb.TxStatus_TX_STATUS_PENDING,
			Confirmations:         0,
			RequiredConfirmations: 3,
			IsConfirmed:           false,
			BlockNumber:           "",
		}, nil
	}

	// Get current block number for confirmation count
	currentBlock, err := client.BlockNumber(l.ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get current block number: %v", err)
	}

	// Calculate confirmations
	var confirmations uint64
	if receipt.BlockNumber != nil {
		confirmations = currentBlock - receipt.BlockNumber.Uint64()
	}

	// Determine transaction status
	var status pb.TxStatus
	if receipt.Status == 1 {
		status = pb.TxStatus_TX_STATUS_CONFIRMED
	} else {
		status = pb.TxStatus_TX_STATUS_FAILED
	}

	// Set required confirmations
	requiredConfirmations := uint64(3)

	return &pb.GetTransactionStatusResp{
		Success:               true,
		Message:               "Transaction status retrieved",
		Status:                status,
		Confirmations:         confirmations,
		RequiredConfirmations: requiredConfirmations,
		IsConfirmed:           confirmations >= requiredConfirmations,
		BlockNumber:           receipt.BlockNumber.String(),
	}, nil
}

// getTronTransactionStatus 查询 TRON 交易状态
func (l *GetTransactionStatusLogic) getTronTransactionStatus(in *pb.GetTransactionStatusReq) (*pb.GetTransactionStatusResp, error) {
	// Get TRON client
	tronClient, err := l.svcCtx.ChainMgr.GetTronClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get TRON client: %v", err)
	}
	defer tronClient.Stop()

	// Get current block number for confirmation count
	nowBlock, err := tronClient.GetNowBlock()
	if err != nil {
		return nil, fmt.Errorf("failed to get current block: %v", err)
	}
	var currentBlock int64
	if nowBlock != nil && nowBlock.BlockHeader != nil && nowBlock.BlockHeader.RawData != nil {
		currentBlock = nowBlock.BlockHeader.RawData.Number
	}

	// Try to get transaction info first (includes block number and status)
	txInfo, err := tronClient.GetTransactionInfoByID(in.TxHash)
	if err == nil && txInfo != nil {
		// Transaction is confirmed, we have all info we need
		var status pb.TxStatus
		if txInfo.Result == 0 { // TransactionInfoCode: 0 = SUCCESS, 1 = FAILED
			status = pb.TxStatus_TX_STATUS_CONFIRMED
		} else {
			status = pb.TxStatus_TX_STATUS_FAILED
		}

		// Calculate confirmations
		var confirmations uint64
		if txInfo.BlockNumber > 0 && currentBlock > 0 {
			if currentBlock > txInfo.BlockNumber {
				confirmations = uint64(currentBlock - txInfo.BlockNumber)
			} else {
				confirmations = 0
			}
		}

		// Required confirmations
		requiredConfirmations := uint64(3)

		return &pb.GetTransactionStatusResp{
			Success:               true,
			Message:               "TRON transaction status retrieved",
			Status:                status,
			Confirmations:         confirmations,
			RequiredConfirmations: requiredConfirmations,
			IsConfirmed:           confirmations >= requiredConfirmations,
			BlockNumber:           fmt.Sprintf("%d", txInfo.BlockNumber),
		}, nil
	}

	//如果上面未查询到数据，则使用GetTransactionByID 查询
	tx, err := tronClient.GetTransactionByID(in.TxHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get TRON transaction: %v", err)
	}

	if tx == nil || tx.RawData == nil {
		return &pb.GetTransactionStatusResp{
			Success:               true,
			Message:               "Transaction not found",
			Status:                pb.TxStatus_TX_STATUS_PENDING,
			Confirmations:         0,
			RequiredConfirmations: 3,
			IsConfirmed:           false,
		}, nil
	}

	// Determine transaction status from transaction data
	var status pb.TxStatus
	if len(tx.Ret) > 0 && tx.Ret[0].Ret == 0 {
		status = pb.TxStatus_TX_STATUS_CONFIRMED
	} else {
		status = pb.TxStatus_TX_STATUS_FAILED
	}

	var txBlockNum int64 = 0
	var confirmations uint64 = 0

	if currentBlock > 0 {
		confirmations = uint64(currentBlock - txBlockNum)
	}

	// Required confirmations
	requiredConfirmations := uint64(3)

	return &pb.GetTransactionStatusResp{
		Success:               true,
		Message:               "TRON transaction status retrieved",
		Status:                status,
		Confirmations:         confirmations,
		RequiredConfirmations: requiredConfirmations,
		IsConfirmed:           confirmations >= requiredConfirmations,
		BlockNumber:           fmt.Sprintf("%d", txBlockNum),
	}, nil
}
