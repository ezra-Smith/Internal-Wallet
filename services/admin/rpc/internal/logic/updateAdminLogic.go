package logic

import (
	"context"
	"encoding/json"
	"strings"

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

type UpdateAdminLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateAdminLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateAdminLogic {
	return &UpdateAdminLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateAdminLogic) UpdateAdmin(in *pb.UpdateAdminRequest) (*pb.UpdateAdminResponse, error) {
	if in == nil || strings.TrimSpace(in.AdminId) == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid params", map[string]string{"admin_id": "required"})
	}
	if l.svcCtx.DB == nil || l.svcCtx.AdminUserRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	targetID, parseErr := resp.ParseAdminID(in.AdminId)
	if parseErr != nil || targetID <= 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid admin_id", map[string]string{"admin_id": "invalid"})
	}

	current, _ := admininterceptor.GetCurrentAdmin(l.ctx)
	operatorID := int64(0)
	operatorUsername := ""
	if current != nil {
		operatorID = current.ID
		operatorUsername = current.Username
	}

	target, err := l.svcCtx.AdminUserRepo.FindByID(l.ctx, targetID)
	if err != nil || target == nil {
		return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "NOT_FOUND", "admin not found", nil)
	}

	updates := map[string]interface{}{}
	changes := map[string]interface{}{}

	if name := strings.TrimSpace(in.Name); name != "" && name != target.Name {
		updates["name"] = name
		changes["name"] = map[string]string{"old": target.Name, "new": name}
	}

	// Role 已迁移至 RBAC v2（admin_user_role），请使用 AssignRolesToUser 接口调整。
	if role := strings.TrimSpace(in.Role); role != "" && role != target.Role {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "ROLE_UPDATE_NOT_SUPPORTED", "角色更新已迁移到 RBAC v2，请使用角色分配接口", map[string]string{"role": "use AssignRolesToUser"})
	}

	if status := strings.TrimSpace(in.Status); status != "" && status != target.Status {
		// 禁用必须走 DisableAdmin（需要 reason + 幂等键）
		if status == "disabled" {
			return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_STATUS", "禁用请使用 disable 接口", map[string]string{"status": "use disable endpoint"})
		}
		if status != "active" {
			return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_STATUS", "无效状态", map[string]string{"status": "invalid"})
		}
		updates["status"] = status
		changes["status"] = map[string]string{"old": target.Status, "new": status}
	}

	if len(updates) == 0 {
		return &pb.UpdateAdminResponse{
			Success: true,
			Message: resp.Msg(l.ctx, "NO_CHANGES"),
			Data: &pb.UpdateAdminData{
				AdminId:   resp.AdminIDString(target.ID),
				Name:      target.Name,
				Role:      target.Role,
				UpdatedAt: formatTime(target.UpdatedAt),
				UpdatedBy: operatorUsername,
			},
			RequestId: resp.RequestID(l.ctx),
			Timestamp: resp.Timestamp(),
		}, nil
	}

	updates["updated_by"] = operatorID

	ip := middleware.GetClientIP(l.ctx)
	ua := middleware.GetUserAgent(l.ctx)
	detailsBytes, _ := json.Marshal(map[string]interface{}{
		"admin_id": resp.AdminIDString(targetID),
		"changes":  changes,
		"operator": operatorUsername,
	})

	needRevoke := false
	if _, ok := updates["status"]; ok {
		needRevoke = true
	}

	if err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		adminRepo := repository.NewAdminUserRepository(tx)
		auditRepo := repository.NewAdminAuditLogRepository(tx)

		if err := adminRepo.UpdateFields(l.ctx, targetID, updates); err != nil {
			return err
		}
		if needRevoke {
			if _, err := adminRepo.IncrementTokenVersion(l.ctx, targetID, operatorID); err != nil {
				return err
			}
		}
		_ = auditRepo.CreateLog(l.ctx, &model.AdminAuditLogModel{
			AdminID:     operatorID,
			Action:      "admin.update",
			TargetType:  "admin",
			TargetID:    resp.AdminIDString(targetID),
			Description: "更新管理员信息",
			Details:     detailsBytes,
			IP:          ip,
			UserAgent:   ua,
		})
		return nil
	}); err != nil {
		l.Logger.Errorf("update admin failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	// 权限/状态变更后踢下线
	if needRevoke {
		_ = l.svcCtx.SessionManager.RemoveAll(l.ctx, targetID)
	}

	updated, _ := l.svcCtx.AdminUserRepo.FindByID(l.ctx, targetID)
	if updated == nil {
		updated = target
	}

	return &pb.UpdateAdminResponse{
		Success: true,
		Message: resp.Msg(l.ctx, "ADMIN_UPDATED"),
		Data: &pb.UpdateAdminData{
			AdminId:   resp.AdminIDString(targetID),
			Name:      updated.Name,
			Role:      updated.Role,
			UpdatedAt: formatTime(updated.UpdatedAt),
			UpdatedBy: operatorUsername,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
