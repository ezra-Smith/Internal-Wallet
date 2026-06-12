package logic

import (
	"context"
	"fmt"
	"time"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type RefreshTokenLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRefreshTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RefreshTokenLogic {
	return &RefreshTokenLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// RefreshToken 刷新访问令牌（服务器端会话验证 + 令牌轮换）
func (l *RefreshTokenLogic) RefreshToken(in *pb.RefreshTokenReq) (*pb.RefreshTokenResp, error) {
	// 1. 参数验证
	if in == nil || in.RefreshToken == "" {
		return nil, errx.InvalidParam("invalid params")
	}

	// 2. 服务器端会话验证：查找 refresh token 是否存在且有效
	if l.svcCtx.UserSessionRepository == nil {
		l.Logger.Error("UserSessionRepository not initialized")
		return nil, errx.Internal("internal error")
	}

	session, err := l.svcCtx.UserSessionRepository.FindByRefreshToken(l.ctx, in.RefreshToken)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			l.Logger.Infof("Invalid refresh token: not found in database")
			return nil, errx.RefreshTokenInvalid()
		}
		l.Logger.Errorf("Failed to find session by refresh token: %v", err)
		return nil, errx.Internal("internal error")
	}

	// 3. 验证会话是否有效
	if !session.IsActive {
		l.Logger.Infof("Refresh token from inactive session (user_id: %d, session_id: %d)", session.UserId, session.ID)
		return nil, errx.SessionTerminated()
	}

	if session.ExpiresAt.Before(time.Now()) {
		l.Logger.Infof("Refresh token expired (user_id: %d, session_id: %d, expired_at: %v)",
			session.UserId, session.ID, session.ExpiresAt)
		return nil, errx.RefreshTokenExpired()
	}

	// 4. 获取用户信息
	user, err := l.svcCtx.UserAccountRepository.GetAuthByID(l.ctx, session.UserId)
	if err != nil {
		l.Logger.Errorf("User not found for session (user_id: %d): %v", session.UserId, err)
		return nil, errx.UserNotFound()
	}

	uid := fmt.Sprintf("%d", user.ID)
	email := user.Email
	if email == "" {
		email = user.Phone // 如果email为空，使用phone
	}

	// 5. 生成新的访问令牌
	newAccessToken, err := middleware.GenerateToken(
		uid,
		email,
		l.svcCtx.Config.JWT.AccessSecret,
		time.Duration(l.svcCtx.Config.JWT.AccessExpire)*time.Second,
	)
	if err != nil {
		l.Logger.Errorf("Failed to generate access token: %v", err)
		return nil, errx.TokenGenerateFailed()
	}

	// 6. 令牌轮换：生成新的 refresh token
	newRefreshToken, err := middleware.GenerateToken(
		uid,
		email,
		l.svcCtx.Config.JWT.RefreshSecret,
		time.Duration(l.svcCtx.Config.JWT.RefreshExpire)*time.Second,
	)
	if err != nil {
		l.Logger.Errorf("Failed to generate refresh token: %v", err)
		return nil, errx.TokenGenerateFailed()
	}

	// 7. 更新数据库中的令牌（令牌轮换）
	newExpiresAt := time.Now().Add(time.Duration(l.svcCtx.Config.JWT.RefreshExpire) * time.Second)
	err = l.svcCtx.UserSessionRepository.UpdateTokens(
		l.ctx,
		session.ID,
		newAccessToken,  // session_token
		newRefreshToken, // refresh_token
		newExpiresAt,
	)
	if err != nil {
		l.Logger.Errorf("Failed to update session tokens (session_id: %d): %v", session.ID, err)
		return nil, errx.SessionUpdateFailed()
	}

	l.Logger.Infof("Token refreshed successfully for user %d (session_id: %d)", session.UserId, session.ID)

	return &pb.RefreshTokenResp{
		Success:      true,
		Message:      "ok",
		AccessToken:  newAccessToken,
		RefreshToken: newRefreshToken,
		ExpiresIn:    int64(l.svcCtx.Config.JWT.AccessExpire),
	}, nil
}
