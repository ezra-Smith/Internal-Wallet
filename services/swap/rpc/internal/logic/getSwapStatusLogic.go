package logic

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/swap/rpc/internal/svc"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetSwapStatusLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetSwapStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSwapStatusLogic {
	return &GetSwapStatusLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetSwapStatusLogic) GetSwapStatus(in *pb.GetSwapStatusRequest) (*pb.GetSwapStatusResponse, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if in.ChainId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "chain_id is required")
	}
	txHash := strings.TrimSpace(in.TxHash)
	swapIDStr := strings.TrimSpace(in.SwapId)

	var providerName string
	var recordID int64
	if swapIDStr != "" && l.svcCtx.TxRepo != nil {
		id, err := strconv.ParseInt(swapIDStr, 10, 64)
		if err != nil || id <= 0 {
			return nil, status.Error(codes.InvalidArgument, "invalid swap_id")
		}
		recordID = id
		rec, err := l.svcCtx.TxRepo.FindByID(l.ctx, id)
		if err == nil && rec != nil {
			providerName = strings.TrimSpace(rec.Provider)
			if txHash == "" {
				txHash = strings.TrimSpace(rec.TxHash)
			} else if strings.TrimSpace(rec.TxHash) == "" {
				_ = l.svcCtx.TxRepo.UpdateFields(l.ctx, id, map[string]any{
					"tx_hash": strings.TrimSpace(txHash),
				})
			}
		}
	}

	if txHash == "" {
		return nil, status.Error(codes.InvalidArgument, "tx_hash is required")
	}

	requiredConfs, _ := l.svcCtx.EVM.GetRequiredConfirmations(in.ChainId)
	if requiredConfs == 0 {
		requiredConfs = 12
	}

	cli, err := l.svcCtx.EVM.GetClient(l.ctx, in.ChainId)
	if err != nil {
		l.Logger.Errorw("get evm client failed", logx.Field("error", err), logx.Field("chain_id", in.ChainId))
		return nil, status.Error(codes.Unavailable, "chain rpc not available")
	}

	hash := common.HexToHash(txHash)
	receipt, err := cli.TransactionReceipt(l.ctx, hash)
	statusEnum := pb.SwapTxStatus_SWAP_TX_STATUS_PENDING
	var confirmations uint64
	var blockNumber string

	if err != nil {
		if errors.Is(err, ethereum.NotFound) || strings.Contains(strings.ToLower(err.Error()), "not found") {
			statusEnum = pb.SwapTxStatus_SWAP_TX_STATUS_PENDING
		} else {
			l.Logger.Errorw("query receipt failed", logx.Field("error", err), logx.Field("chain_id", in.ChainId), logx.Field("tx_hash", txHash))
			return nil, status.Error(codes.Unavailable, "failed to query transaction receipt")
		}
	} else if receipt != nil && receipt.BlockNumber != nil {
		currentBlock, err := cli.BlockNumber(l.ctx)
		if err != nil {
			l.Logger.Errorw("query current block failed", logx.Field("error", err), logx.Field("chain_id", in.ChainId))
			return nil, status.Error(codes.Unavailable, "failed to query latest block")
		}
		if currentBlock >= receipt.BlockNumber.Uint64() {
			confirmations = currentBlock - receipt.BlockNumber.Uint64()
		}
		blockNumber = receipt.BlockNumber.String()
		if receipt.Status == 1 {
			statusEnum = pb.SwapTxStatus_SWAP_TX_STATUS_SUCCESS
		} else {
			statusEnum = pb.SwapTxStatus_SWAP_TX_STATUS_FAILED
		}
	}

	isConfirmed := confirmations >= requiredConfs && (statusEnum == pb.SwapTxStatus_SWAP_TX_STATUS_SUCCESS || statusEnum == pb.SwapTxStatus_SWAP_TX_STATUS_FAILED)

	// Best-effort DB status update.
	if recordID > 0 && l.svcCtx.TxRepo != nil {
		dbStatus := "pending"
		if statusEnum == pb.SwapTxStatus_SWAP_TX_STATUS_SUCCESS {
			dbStatus = "success"
		} else if statusEnum == pb.SwapTxStatus_SWAP_TX_STATUS_FAILED {
			dbStatus = "failed"
		}
		_ = l.svcCtx.TxRepo.UpdateFields(l.ctx, recordID, map[string]any{
			"status":     dbStatus,
			"tx_hash":    txHash,
			"updated_at": time.Now(),
		})
	}

	return &pb.GetSwapStatusResponse{
		Success: true,
		Message: "ok",
		Data: &pb.SwapStatusData{
			SwapId:                swapIDStr,
			ChainId:               in.ChainId,
			TxHash:                txHash,
			Status:                statusEnum,
			Confirmations:         confirmations,
			RequiredConfirmations: requiredConfs,
			IsConfirmed:           isConfirmed,
			BlockNumber:           blockNumber,
			Provider:              providerName,
		},
	}, nil
}
