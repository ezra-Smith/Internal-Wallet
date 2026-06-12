package logic

import (
	"context"
	"strings"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/swap/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UpdateSwapBroadcastResultLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateSwapBroadcastResultLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateSwapBroadcastResultLogic {
	return &UpdateSwapBroadcastResultLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// UpdateSwapBroadcastResult updates swap_svc_transactions after an external broadcast (e.g. client broadcast via /web3/transaction/broadcast).
func (l *UpdateSwapBroadcastResultLogic) UpdateSwapBroadcastResult(in *pb.UpdateSwapBroadcastResultRequest) (*pb.UpdateSwapBroadcastResultResponse, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if l.svcCtx.TxRepo == nil {
		return nil, status.Error(codes.Internal, "database not configured")
	}

	swapID, err := parseSwapID(in.SwapId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid swap_id")
	}

	txHash := strings.TrimSpace(in.TxHash)
	if txHash == "" {
		return nil, status.Error(codes.InvalidArgument, "tx_hash is required")
	}

	// Ensure record exists and (optionally) validate chain_id.
	swapRecord, err := l.svcCtx.TxRepo.FindByID(l.ctx, swapID)
	if err != nil || swapRecord == nil {
		return nil, status.Error(codes.NotFound, "swap record not found")
	}
	if in.ChainId > 0 && swapRecord.ChainID != in.ChainId {
		return nil, status.Error(codes.InvalidArgument, "chain_id mismatch")
	}

	if in.BroadcastSuccess {
		now := time.Now().Local()
		if in.BroadcastedAt > 0 {
			now = time.Unix(in.BroadcastedAt, 0).Local()
		}

		updateFields := map[string]any{
			"status":         "pending",
			"tx_hash":        txHash,
			"broadcasted_at": &now,
			"error_code":     "",
			"error_message":  nil,
		}
		if err := l.svcCtx.TxRepo.UpdateFields(l.ctx, swapID, updateFields); err != nil {
			l.Errorw("Failed to update swap record after external broadcast success",
				logx.Field("error", err),
				logx.Field("swap_id", swapID),
				logx.Field("tx_hash", txHash),
			)
			return nil, status.Error(codes.Internal, "failed to update swap record")
		}
		return &pb.UpdateSwapBroadcastResultResponse{Success: true, Message: "ok"}, nil
	}

	errCode := strings.TrimSpace(in.ErrorCode)
	if errCode == "" {
		errCode = "BROADCAST_FAILED"
	}
	errMsg := strings.TrimSpace(in.ErrorMessage)
	var errMsgPtr *string
	if errMsg != "" {
		errMsgPtr = &errMsg
	}

	updateFields := map[string]any{
		"status":        "failed",
		"tx_hash":       txHash,
		"error_code":    errCode,
		"error_message": errMsgPtr,
	}

	if err := l.svcCtx.TxRepo.UpdateFields(l.ctx, swapID, updateFields); err != nil {
		l.Errorw("Failed to update swap record after external broadcast failure",
			logx.Field("error", err),
			logx.Field("swap_id", swapID),
			logx.Field("tx_hash", txHash),
		)
		return nil, status.Error(codes.Internal, "failed to update swap record")
	}

	return &pb.UpdateSwapBroadcastResultResponse{Success: true, Message: "ok"}, nil
}
