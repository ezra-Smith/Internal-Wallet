package logic

import (
	"context"
	"encoding/json"
	"sort"
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
	"gorm.io/gorm/clause"
)

type AssignRolesToUserLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAssignRolesToUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AssignRolesToUserLogic {
	return &AssignRolesToUserLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AssignRolesToUserLogic) AssignRolesToUser(in *pb.AssignRolesToUserRequest) (*pb.AssignRolesToUserResponse, error) {
	if in == nil || in.UserId <= 0 || len(in.RoleIds) == 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid params", map[string]string{
			"user_id":  "required",
			"role_ids": "required",
		})
	}
	if l.svcCtx.DB == nil || l.svcCtx.AdminUserRepo == nil || l.svcCtx.AdminRoleRepo == nil || l.svcCtx.AdminRBACRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	current, _ := admininterceptor.GetCurrentAdmin(l.ctx)
	operatorID := int64(0)
	operatorUsername := ""
	if current != nil {
		operatorID = current.ID
		operatorUsername = current.Username
	}

	user, err := l.svcCtx.AdminUserRepo.FindByID(l.ctx, in.UserId)
	if err != nil || user == nil {
		return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "NOT_FOUND", "user not found", nil)
	}

	roleIDs := uniqueInt64s(in.RoleIds)
	if len(roleIDs) == 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid role_ids", map[string]string{"role_ids": "invalid"})
	}

	roles, err := l.svcCtx.AdminRoleRepo.FindByIDs(l.ctx, roleIDs)
	if err != nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "check roles failed", nil)
	}
	if len(roles) != len(roleIDs) {
		return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "NOT_FOUND", "some roles not found", nil)
	}
	for _, r := range roles {
		if r == nil || r.Status != 1 {
			return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_ROLE", "role disabled", map[string]string{"role_ids": "contains disabled"})
		}
	}

	replace := true
	if in.Replace != nil {
		replace = in.Replace.Value
	}

	// RBAC guard: cannot remove the last active super_admin.
	if replace && user.Status == "active" {
		currentRoles, err := l.svcCtx.AdminRBACRepo.GetUserRoles(l.ctx, in.UserId)
		if err != nil {
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "query user roles failed", nil)
		}
		hasSuperBefore := false
		for _, r := range currentRoles {
			if r != nil && r.Code == "super_admin" {
				hasSuperBefore = true
				break
			}
		}
		hasSuperAfter := false
		for _, r := range roles {
			if r != nil && r.Code == "super_admin" {
				hasSuperAfter = true
				break
			}
		}
		if hasSuperBefore && !hasSuperAfter {
			cnt, cErr := l.svcCtx.AdminUserRepo.CountActiveSuperAdmins(l.ctx)
			if cErr != nil {
				return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
			}
			if cnt <= 1 {
				return nil, errx.New(codes.InvalidArgument, 400, errx.CodeLastSuperAdmin, "LAST_SUPER_ADMIN", "不能移除最后一个超级管理员", nil)
			}
		}
	}

	// Keep admin_users.role in sync as "primary role" for compatibility.
	sort.Slice(roles, func(i, j int) bool {
		if roles[i] == nil {
			return false
		}
		if roles[j] == nil {
			return true
		}
		return roles[i].ID < roles[j].ID
	})
	primaryRoleCode := ""
	if len(roles) > 0 && roles[0] != nil {
		primaryRoleCode = roles[0].Code
	}

	now := time.Now()
	if err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		adminRepo := repository.NewAdminUserRepository(tx)

		if replace {
			if err := tx.WithContext(l.ctx).Where("user_id = ?", in.UserId).Delete(&model.AdminUserRoleModel{}).Error; err != nil {
				return errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "cleanup user roles failed", nil)
			}
		}

		rows := make([]*model.AdminUserRoleModel, 0, len(roleIDs))
		for _, rid := range roleIDs {
			rid := rid
			rows = append(rows, &model.AdminUserRoleModel{
				UserID:    in.UserId,
				RoleID:    rid,
				CreatedAt: &now,
			})
		}
		if err := tx.WithContext(l.ctx).
			Clauses(clause.OnConflict{DoNothing: true}).
			CreateInBatches(rows, 200).Error; err != nil {
			return errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "assign user roles failed", nil)
		}

		if primaryRoleCode != "" {
			if err := adminRepo.UpdateRole(l.ctx, in.UserId, primaryRoleCode, operatorID); err != nil {
				return err
			}
		}
		if _, err := adminRepo.IncrementTokenVersion(l.ctx, in.UserId, operatorID); err != nil {
			return errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "revoke tokens failed", nil)
		}

		// Best-effort audit log
		ip := middleware.GetClientIP(l.ctx)
		ua := middleware.GetUserAgent(l.ctx)
		detailsBytes, _ := json.Marshal(map[string]interface{}{
			"user_id":      in.UserId,
			"role_ids":     roleIDs,
			"replace":      replace,
			"primary_role": primaryRoleCode,
			"operator":     operatorUsername,
		})
		_ = tx.WithContext(l.ctx).Create(&model.AdminAuditLogModel{
			AdminID:     operatorID,
			Action:      "user.roles.assign",
			TargetType:  "admin",
			TargetID:    resp.AdminIDString(in.UserId),
			Description: "分配用户角色",
			Details:     detailsBytes,
			IP:          ip,
			UserAgent:   ua,
		}).Error
		return nil
	}); err != nil {
		return nil, err
	}

	// Kick sessions after role change (best-effort).
	_ = l.svcCtx.SessionManager.RemoveAll(l.ctx, in.UserId)

	return &pb.AssignRolesToUserResponse{
		Success: true,
		Message: "ok",
		Data: &pb.AssignRolesToUserData{
			UserId:        in.UserId,
			AssignedCount: int32(len(roleIDs)),
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
