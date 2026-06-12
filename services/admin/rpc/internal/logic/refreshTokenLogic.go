package logic

import (
	"context"
	"time"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	admininterceptor "internalwallet/services/admin/rpc/internal/interceptor"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/security"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
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

func (l *RefreshTokenLogic) RefreshToken(in *pb.RefreshTokenRequest) (*pb.RefreshTokenResponse, error) {
	_ = in
	if l.svcCtx.AdminUserRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "service not ready", nil)
	}

	admin, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || admin == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	jti := middleware.GetJWTID(l.ctx)
	if jti == "" {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}
	expUnix := middleware.GetJWTExpiresAt(l.ctx)
	if expUnix <= 0 {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}
	now := time.Now()
	remaining := expUnix - now.Unix()
	if remaining <= 0 {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_EXPIRED", "token expired", nil)
	}

	window := l.svcCtx.Config.Security.RefreshWindowSeconds
	if window <= 0 {
		window = 2 * 60 * 60
	}
	if remaining > window {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeAuthRefreshNotAllowed, "AUTH_REFRESH_NOT_ALLOWED", "Token 有效期充足，无需刷新", nil)
	}

	// 每个 token 只能刷新一次
	if l.svcCtx.TokenRefresh != nil {
		refreshed, err := l.svcCtx.TokenRefresh.IsRefreshed(l.ctx, jti)
		if err != nil {
			l.Logger.Errorf("check token refreshed failed: %v", err)
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
		}
		if refreshed {
			return nil, errx.New(codes.InvalidArgument, 400, errx.CodeAuthRefreshNotAllowed, "AUTH_REFRESH_NOT_ALLOWED", "Token 已刷新，请重新登录", nil)
		}
	}

	expireSeconds := int64(28800)
	if l.svcCtx.Config.JWT.AccessExpire > 0 {
		expireSeconds = l.svcCtx.Config.JWT.AccessExpire
	}
	accessToken, meta, genErr := security.GenerateAdminAccessToken(admin.ID, admin.Username, admin.TokenVersion, l.svcCtx.Config.JWT.AccessSecret, expireSeconds)
	if genErr != nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "token generate failed", nil)
	}

	// 标记 refreshed（使用旧 token 的剩余 TTL）
	if l.svcCtx.TokenRefresh != nil {
		ttl := time.Until(time.Unix(expUnix, 0))
		if ttl < 0 {
			ttl = time.Minute
		}
		if err := l.svcCtx.TokenRefresh.MarkRefreshed(l.ctx, jti, ttl); err != nil {
			l.Logger.Errorf("mark refreshed failed: %v", err)
		}
	}

	// 旧 token 立即失效：移除 session + 加入黑名单
	ttl := time.Until(time.Unix(expUnix, 0))
	_ = l.svcCtx.SessionManager.Remove(l.ctx, admin.ID, jti)
	_ = l.svcCtx.TokenBlacklist.Blacklist(l.ctx, jti, ttl)

	// 写入新 session
	_ = l.svcCtx.SessionManager.Add(l.ctx, admin.ID, meta.JTI, meta.Exp)

	return &pb.RefreshTokenResponse{
		Success: true,
		Message: resp.Msg(l.ctx, "TOKEN_REFRESHED"),
		Data: &pb.RefreshTokenData{
			AccessToken: accessToken,
			ExpiresIn:   expireSeconds,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
