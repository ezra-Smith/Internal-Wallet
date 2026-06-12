package logic

import (
	"context"
	"encoding/json"
	"strconv"
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

type UpdateUserRoleLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateUserRoleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateUserRoleLogic {
	return &UpdateUserRoleLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateUserRoleLogic) UpdateUserRole(in *pb.UpdateUserRoleRequest) (*pb.UpdateUserRoleResponse, error) {
	if in == nil || strings.TrimSpace(in.Uid) == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_UID", "uid required", map[string]string{"uid": "required"})
	}
	newRole := strings.TrimSpace(in.Role)
	if newRole != "user" && newRole != "vip" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_ROLE", "无效角色", map[string]string{"role": "invalid"})
	}
	if l.svcCtx.DB == nil || l.svcCtx.UserRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	uidStr := strings.TrimSpace(in.Uid)
	uid, err := strconv.ParseInt(uidStr, 10, 64)
	if err != nil || uid <= 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_UID", "invalid uid", map[string]string{"uid": "invalid"})
	}

	u, err := l.svcCtx.UserRepo.FindByID(l.ctx, uid)
	if err != nil || u == nil {
		return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "USER_NOT_FOUND", "用户不存在", nil)
	}

	oldRole := userRoleFromMemberLevel(u.MemberLevel)
	if oldRole == newRole {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "SAME_ROLE", "新角色与当前角色相同", map[string]string{"role": "same"})
	}

	var newMemberLevel int32 = 1
	if newRole == "vip" {
		newMemberLevel = 2
	}

	now := time.Now()
	ip := middleware.GetClientIP(l.ctx)
	ua := middleware.GetUserAgent(l.ctx)
	detailsBytes, _ := json.Marshal(map[string]interface{}{
		"old_role": oldRole,
		"new_role": newRole,
		"reason":   strings.TrimSpace(in.Reason),
	})

	if err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		userRepo := repository.NewUserRepository(tx)
		auditRepo := repository.NewAdminAuditLogRepository(tx)

		if err := userRepo.UpdateMemberLevel(l.ctx, uid, newMemberLevel); err != nil {
			return err
		}
		_ = auditRepo.CreateLog(l.ctx, &model.AdminAuditLogModel{
			AdminID:     current.ID,
			Action:      "user.update_role",
			TargetType:  "user",
			TargetID:    uidStr,
			Description: "更新用户角色: " + uidStr,
			Details:     detailsBytes,
			IP:          ip,
			UserAgent:   ua,
		})
		return nil
	}); err != nil {
		l.Logger.Errorf("update user role failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	return &pb.UpdateUserRoleResponse{
		Success: true,
		Message: resp.Msg(l.ctx, "ROLE_UPDATED"),
		Data: &pb.UpdateUserRoleData{
			Uid:       uidStr,
			OldRole:   oldRole,
			NewRole:   newRole,
			UpdatedAt: formatTime(now),
			UpdatedBy: current.Username,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
