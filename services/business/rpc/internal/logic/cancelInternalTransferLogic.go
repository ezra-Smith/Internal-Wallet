package logic

import (
	"context"
	"strconv"
	"time"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/model"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CancelInternalTransferLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCancelInternalTransferLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CancelInternalTransferLogic {
	return &CancelInternalTransferLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CancelInternalTransferLogic) CancelInternalTransfer(in *pb.CancelInternalTransferReq) (*pb.CancelInternalTransferResp, error) {
	// Get current user
	uidStr := middleware.GetUserID(l.ctx)
	if uidStr == "" {
		return nil, errx.Unauthorized("unauthorized")
	}
	userID, _ := strconv.ParseInt(uidStr, 10, 64)
	if userID == 0 {
		return nil, errx.Unauthorized("unauthorized")
	}

	// Parse order ID
	orderID, err := strconv.ParseInt(in.OrderId, 10, 64)
	if err != nil || orderID == 0 {
		return nil, errx.InvalidParam("invalid order_id")
	}

	// Get order
	order, err := l.svcCtx.CurrencyTransferOrderRepository.GetByID(l.ctx, orderID)
	if err != nil {
		l.Logger.Errorf("Failed to get transfer order: %v", err)
		return nil, errx.NotFound("transfer order not found")
	}
	if order == nil {
		return nil, errx.NotFound("transfer order not found")
	}

	// Check permission - only sender can cancel
	if order.FromUserID != userID {
		return nil, errx.Forbidden("only sender can cancel transfer")
	}

	// Check if can be cancelled - only pending orders
	if order.Status != model.TransferStatusPending {
		return nil, errx.InvalidParam("only pending transfers can be cancelled")
	}

	// Cancel the order
	now := time.Now()
	order.Status = model.TransferStatusCancelled
	order.AuditedAt = &now
	cancelNote := "cancelled by user"
	order.AuditNote = &cancelNote
	order.UpdatedBy = userID

	if err := l.svcCtx.CurrencyTransferOrderRepository.Update(l.ctx, order); err != nil {
		l.Logger.Errorf("Failed to cancel transfer order: %v", err)
		return nil, errx.Internal("failed to cancel transfer")
	}

	return &pb.CancelInternalTransferResp{
		Success: true,
		Message: "transfer cancelled",
	}, nil
}
