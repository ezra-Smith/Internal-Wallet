package logic

import (
	"context"
	"time"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	admininterceptor "internalwallet/services/admin/rpc/internal/interceptor"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type LogoutLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLogoutLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LogoutLogic {
	return &LogoutLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *LogoutLogic) Logout(in *pb.LogoutRequest) (*pb.LogoutResponse, error) {
	_ = in
	admin, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || admin == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	jti := middleware.GetJWTID(l.ctx)
	if jti == "" {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}
	expUnix := middleware.GetJWTExpiresAt(l.ctx)
	ttl := time.Until(time.Unix(expUnix, 0))
	if ttl < 0 {
		ttl = time.Minute
	}

	// 移除服务端会话 + 加入黑名单
	_ = l.svcCtx.SessionManager.Remove(l.ctx, admin.ID, jti)
	_ = l.svcCtx.TokenBlacklist.Blacklist(l.ctx, jti, ttl)

	// 记录登出日志
	ip := middleware.GetClientIP(l.ctx)
	ua := middleware.GetUserAgent(l.ctx)
	if l.svcCtx.AdminLoginLogRepo != nil {
		_ = l.svcCtx.AdminLoginLogRepo.CreateLog(l.ctx, &model.AdminLoginLogModel{
			AdminID:   admin.ID,
			Action:    "logout",
			IP:        ip,
			UserAgent: ua,
			Result:    "success",
		})
	}
	if l.svcCtx.AdminAuditLogRepo != nil {
		_ = l.svcCtx.AdminAuditLogRepo.CreateLog(l.ctx, &model.AdminAuditLogModel{
			AdminID:     admin.ID,
			Action:      "admin.logout",
			TargetType:  "admin",
			TargetID:    resp.AdminIDString(admin.ID),
			Description: "管理员登出",
			Details:     nil,
			IP:          ip,
			UserAgent:   ua,
		})
	}

	return &pb.LogoutResponse{
		Success:   true,
		Message:   resp.Msg(l.ctx, "LOGOUT_SUCCESS"),
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
