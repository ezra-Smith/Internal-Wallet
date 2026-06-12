package logic

import (
	"context"
	"strconv"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetWithdrawInfoLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetWithdrawInfoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetWithdrawInfoLogic {
	return &GetWithdrawInfoLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetWithdrawInfoLogic) GetWithdrawInfo(in *pb.GetWithdrawInfoReq) (*pb.GetWithdrawInfoResp, error) {
	_ = in
	uidStr := middleware.GetUserID(l.ctx)
	if uidStr == "" {
		return nil, errx.Unauthorized("unauthorized")
	}
	userID, _ := strconv.ParseInt(uidStr, 10, 64)
	if userID <= 0 {
		return nil, errx.Unauthorized("unauthorized")
	}

	var hasTradePassword bool
	var hasGoogleAuth bool
	var hasBiometric bool

	if l.svcCtx != nil && l.svcCtx.UserAccountRepository != nil {
		if _, has, err := l.svcCtx.UserAccountRepository.GetTradePasswordHash(l.ctx, userID); err == nil {
			hasTradePassword = has
		}
	}
	if l.svcCtx != nil && l.svcCtx.UserSecuritySettingsRepository != nil {
		if s, err := l.svcCtx.UserSecuritySettingsRepository.GetByUserID(l.ctx, userID); err == nil && s != nil {
			hasGoogleAuth = s.GoogleAuthEnabled
		}
	}
	if l.svcCtx != nil && l.svcCtx.MemberBiometricCredentialRepository != nil {
		if rows, err := l.svcCtx.MemberBiometricCredentialRepository.ListActiveByUser(l.ctx, userID); err == nil && len(rows) > 0 {
			hasBiometric = true
		}
	}

	tip := &pb.SecurityTip{
		HasTradePassword: hasTradePassword,
		HasBiometric:     hasBiometric,
		HasGoogleAuth:    hasGoogleAuth,
	}
	switch {
	case !hasTradePassword:
		tip.RecommendedAction = "建议先设置资金密码"
	case !hasBiometric:
		tip.RecommendedAction = "建议开启生物识别以提升安全性"
	default:
		tip.RecommendedAction = ""
	}

	return &pb.GetWithdrawInfoResp{Success: true, SecurityTip: tip}, nil
}
