package logic

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	admininterceptor "internalwallet/services/admin/rpc/internal/interceptor"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/repository"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"gorm.io/gorm"
)

type ResetAdminTwoFALogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewResetAdminTwoFALogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResetAdminTwoFALogic {
	return &ResetAdminTwoFALogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ResetAdminTwoFALogic) ResetAdminTwoFA(in *pb.ResetAdminTwoFARequest) (*pb.ResetAdminTwoFAResponse, error) {
	if in == nil || strings.TrimSpace(in.AdminId) == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid params", map[string]string{"admin_id": "required"})
	}
	if l.svcCtx.DB == nil || l.svcCtx.AdminUserRepo == nil || l.svcCtx.AdminAuditLogRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	targetID, parseErr := resp.ParseAdminID(in.AdminId)
	if parseErr != nil || targetID <= 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid admin_id", map[string]string{"admin_id": "invalid"})
	}

	target, err := l.svcCtx.AdminUserRepo.FindByID(l.ctx, targetID)
	if err != nil || target == nil {
		return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "NOT_FOUND", "admin not found", nil)
	}

	now := time.Now()
	ip := middleware.GetClientIP(l.ctx)
	ua := middleware.GetUserAgent(l.ctx)

	reason := strings.TrimSpace(in.Reason)
	detailsBytes, _ := json.Marshal(map[string]interface{}{
		"reason":          reason,
		"target_username": target.Username,
		"old_enabled":     target.TwoFactorEnabled,
	})

	if err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		adminRepo := repository.NewAdminUserRepository(tx)
		auditRepo := repository.NewAdminAuditLogRepository(tx)

		if err := adminRepo.ResetTwoFactor(l.ctx, targetID, current.ID); err != nil {
			return err
		}
		if _, err := adminRepo.IncrementTokenVersion(l.ctx, targetID, current.ID); err != nil {
			return err
		}

		_ = auditRepo.CreateLog(l.ctx, &model.AdminAuditLogModel{
			AdminID:     current.ID,
			Action:      "admin.2fa.reset",
			TargetType:  "admin",
			TargetID:    resp.AdminIDString(targetID),
			Description: "重置管理员 2FA: " + target.Username,
			Details:     detailsBytes,
			IP:          ip,
			UserAgent:   ua,
		})
		return nil
	}); err != nil {
		l.Logger.Errorf("reset admin 2fa failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	// 踢掉所有会话（token_version++ 也会强制 access token 失效）
	_ = l.svcCtx.SessionManager.RemoveAll(l.ctx, targetID)

	return &pb.ResetAdminTwoFAResponse{
		Success: true,
		Message: resp.Msg(l.ctx, "TWO_FA_RESET"),
		Data: &pb.ResetAdminTwoFAData{
			AdminId:          resp.AdminIDString(targetID),
			TwoFactorEnabled: false,
			ResetAt:          formatTime(now),
			ResetBy:          current.Username,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
