package logic

import (
	"context"
	"strconv"
	"strings"

	"internalwallet/common/middleware"
	"internalwallet/common/security"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type UnbindBiometricLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUnbindBiometricLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UnbindBiometricLogic {
	return &UnbindBiometricLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 解绑当前用户生物识别（仅交易密码）
func (l *UnbindBiometricLogic) UnbindBiometric(in *pb.UnbindBiometricReq) (*pb.UnbindBiometricResp, error) {
	if in == nil {
		return nil, errx.InvalidParam("invalid params")
	}
	uidStr := middleware.GetUserID(l.ctx)
	if uidStr == "" {
		return nil, errx.Unauthorized("unauthorized")
	}
	userID, err := strconv.ParseInt(strings.TrimSpace(uidStr), 10, 64)
	if err != nil || userID <= 0 {
		return nil, errx.Unauthorized("unauthorized")
	}

	tradePassword := strings.TrimSpace(in.TradePassword)
	if tradePassword == "" {
		return nil, errx.TradePasswordRequired()
	}

	if l.svcCtx == nil || l.svcCtx.UserAccountRepository == nil || l.svcCtx.MemberBiometricCredentialRepository == nil {
		return nil, errx.Internal("db unavailable")
	}

	tradePasswordHash, hasTradePassword, err := l.svcCtx.UserAccountRepository.GetTradePasswordHash(l.ctx, userID)
	if err != nil {
		l.Logger.Errorf("Failed to get trade password hash: %v", err)
		return nil, errx.Internal("internal error")
	}
	if !hasTradePassword || strings.TrimSpace(tradePasswordHash) == "" {
		return nil, errx.TradePasswordNotSet()
	}
	if !security.VerifyPassword(tradePasswordHash, tradePassword) {
		return nil, errx.InvalidTradePassword()
	}

	if err := l.svcCtx.MemberBiometricCredentialRepository.DisableAllByUser(l.ctx, userID); err != nil {
		l.Logger.Errorf("Disable biometric credentials failed: %v", err)
		return nil, errx.DBError()
	}

	return &pb.UnbindBiometricResp{Success: true, Message: "ok"}, nil
}

