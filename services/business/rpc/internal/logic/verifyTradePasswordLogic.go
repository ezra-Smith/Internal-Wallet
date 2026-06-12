package logic

import (
	"context"
	"strconv"
	"strings"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type VerifyTradePasswordLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewVerifyTradePasswordLogic(ctx context.Context, svcCtx *svc.ServiceContext) *VerifyTradePasswordLogic {
	return &VerifyTradePasswordLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *VerifyTradePasswordLogic) VerifyTradePassword(in *pb.VerifyTradePasswordReq) (*pb.VerifyTradePasswordResp, error) {
	uidStr := middleware.GetUserID(l.ctx)
	if uidStr == "" {
		return nil, errx.Unauthorized("unauthorized")
	}
	id, _ := strconv.ParseInt(uidStr, 10, 64)
	if id <= 0 {
		return nil, errx.Unauthorized("unauthorized")
	}

	tradePassword := strings.TrimSpace(in.TradePassword)
	if tradePassword == "" {
		return nil, errx.TradePasswordRequired()
	}

	tradePasswordHash, hasTradePassword, err := l.svcCtx.UserAccountRepository.GetTradePasswordHash(l.ctx, id)
	if err != nil {
		l.Logger.Errorf("Failed to get trade password hash for user %d: %v", id, err)
		return nil, errx.Internal("internal error")
	}

	if !hasTradePassword || tradePasswordHash == "" {
		return &pb.VerifyTradePasswordResp{
			Success: true,
			Message: "trade password not set",
			IsValid: false,
		}, nil
	}

	// 使用带限制的验证函数
	result, err := VerifyTradePasswordWithLimit(l.ctx, l.svcCtx, id, tradePassword, tradePasswordHash, "verify")
	if err != nil {
		return nil, err
	}

	// 如果账户已锁定
	if result.AccountLocked {
		return &pb.VerifyTradePasswordResp{
			Success:           true,
			Message:           result.Message,
			IsValid:           false,
			RemainingAttempts: int32(result.RemainingAttempts),
			AccountLocked:     true,
		}, nil
	}

	if result.Valid {
		return &pb.VerifyTradePasswordResp{
			Success:           true,
			Message:           "password verified",
			IsValid:           true,
			RemainingAttempts: int32(result.RemainingAttempts),
			AccountLocked:     false,
		}, nil
	}

	return &pb.VerifyTradePasswordResp{
		Success:           true,
		Message:           result.Message,
		IsValid:           false,
		RemainingAttempts: int32(result.RemainingAttempts),
		AccountLocked:     false,
	}, nil
}
