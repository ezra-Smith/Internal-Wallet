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

type UnbindMyWithdrawAuditWhitelistBindingsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUnbindMyWithdrawAuditWhitelistBindingsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UnbindMyWithdrawAuditWhitelistBindingsLogic {
	return &UnbindMyWithdrawAuditWhitelistBindingsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UnbindMyWithdrawAuditWhitelistBindingsLogic) UnbindMyWithdrawAuditWhitelistBindings(in *pb.UnbindMyWithdrawAuditWhitelistBindingsReq) (*pb.UnbindMyWithdrawAuditWhitelistBindingsResp, error) {
	if in == nil {
		return nil, errx.InvalidParam("invalid params")
	}
	uidStr := middleware.GetUserID(l.ctx)
	if uidStr == "" {
		return nil, errx.Unauthorized("unauthorized")
	}
	userID, err := strconv.ParseInt(uidStr, 10, 64)
	if err != nil || userID <= 0 {
		return nil, errx.Unauthorized("unauthorized")
	}

	tradePassword := strings.TrimSpace(in.TradePassword)
	hasTrade := tradePassword != ""
	hasBio := in.Biometric != nil

	if hasTrade == hasBio {
		return nil, errx.InvalidParam("trade_password or biometric required")
	}

	if l.svcCtx == nil || l.svcCtx.UserWithdrawAuditWhitelistRuleRepository == nil || l.svcCtx.UserAccountRepository == nil {
		return nil, errx.Internal("db unavailable")
	}

	if hasTrade {
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
	} else {
		payloadHash, err := calcWithdrawAuditWhitelistUnbindAllPayloadHash()
		if err != nil {
			return nil, errx.Internal("internal error")
		}
		if err := verifyBiometricProof(l.ctx, l.svcCtx, userID, biometricSceneWithdrawAuditWhitelistBind, payloadHash, in.Biometric); err != nil {
			return nil, err
		}
	}

	if err := l.svcCtx.UserWithdrawAuditWhitelistRuleRepository.SoftDeleteByUserSource(l.ctx, userID, "user"); err != nil {
		return nil, errx.DBError()
	}

	return &pb.UnbindMyWithdrawAuditWhitelistBindingsResp{Success: true, Message: "ok"}, nil
}
