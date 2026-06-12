package logic

import (
	"context"
	"encoding/json"
	"strings"
	"time"

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
	"internalwallet/common/middleware"
)

type DisableAdminLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDisableAdminLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DisableAdminLogic {
	return &DisableAdminLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DisableAdminLogic) DisableAdmin(in *pb.DisableAdminRequest) (*pb.DisableAdminResponse, error) {
	if in == nil || strings.TrimSpace(in.AdminId) == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid params", map[string]string{"admin_id": "required"})
	}
	if l.svcCtx.DB == nil || l.svcCtx.AdminUserRepo == nil {
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
	if targetID == current.ID {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeCannotDisableSelf, "CANNOT_DISABLE_SELF", "不能禁用自己", nil)
	}

	target, err := l.svcCtx.AdminUserRepo.FindByID(l.ctx, targetID)
	if err != nil || target == nil {
		return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "NOT_FOUND", "admin not found", nil)
	}

	targetHasSuperAdmin := target.Role == "super_admin"
	if l.svcCtx.AdminRBACRepo != nil {
		if roles, err := l.svcCtx.AdminRBACRepo.GetUserRoles(l.ctx, targetID); err == nil {
			for _, r := range roles {
				if r != nil && r.Code == "super_admin" {
					targetHasSuperAdmin = true
					break
				}
			}
		}
	}
	if targetHasSuperAdmin && target.Status == "active" {
		cnt, cErr := l.svcCtx.AdminUserRepo.CountActiveSuperAdmins(l.ctx)
		if cErr != nil {
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
		}
		if cnt <= 1 {
			return nil, errx.New(codes.InvalidArgument, 400, errx.CodeLastSuperAdmin, "LAST_SUPER_ADMIN", "不能禁用最后一个超级管理员", nil)
		}
	}

	now := time.Now()
	ip := middleware.GetClientIP(l.ctx)
	ua := middleware.GetUserAgent(l.ctx)
	detailsBytes, _ := json.Marshal(map[string]interface{}{
		"reason": strings.TrimSpace(in.Reason),
	})

	if target.Status != "disabled" {
		if err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
			adminRepo := repository.NewAdminUserRepository(tx)
			auditRepo := repository.NewAdminAuditLogRepository(tx)

			if err := adminRepo.UpdateStatus(l.ctx, targetID, "disabled", current.ID); err != nil {
				return err
			}
			if _, err := adminRepo.IncrementTokenVersion(l.ctx, targetID, current.ID); err != nil {
				return err
			}
			_ = auditRepo.CreateLog(l.ctx, &model.AdminAuditLogModel{
				AdminID:     current.ID,
				Action:      "admin.disable",
				TargetType:  "admin",
				TargetID:    resp.AdminIDString(targetID),
				Description: "禁用管理员: " + target.Username,
				Details:     detailsBytes,
				IP:          ip,
				UserAgent:   ua,
			})
			return nil
		}); err != nil {
			l.Logger.Errorf("disable admin failed: %v", err)
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
		}

		// 禁用后立即使所有会话失效
		_ = l.svcCtx.SessionManager.RemoveAll(l.ctx, targetID)
	}

	return &pb.DisableAdminResponse{
		Success: true,
		Message: resp.Msg(l.ctx, "ADMIN_DISABLED"),
		Data: &pb.DisableAdminData{
			AdminId:    resp.AdminIDString(targetID),
			Status:     "disabled",
			DisabledAt: formatTime(now),
			DisabledBy: current.Username,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
