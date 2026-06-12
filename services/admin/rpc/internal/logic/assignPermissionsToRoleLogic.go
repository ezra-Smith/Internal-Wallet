package logic

import (
	"context"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AssignPermissionsToRoleLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAssignPermissionsToRoleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AssignPermissionsToRoleLogic {
	return &AssignPermissionsToRoleLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AssignPermissionsToRoleLogic) AssignPermissionsToRole(in *pb.AssignPermissionsToRoleRequest) (*pb.AssignPermissionsToRoleResponse, error) {
	if in == nil || in.RoleId <= 0 || len(in.PermissionIds) == 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid params", map[string]string{
			"role_id":        "required",
			"permission_ids": "required",
		})
	}
	if l.svcCtx.DB == nil || l.svcCtx.AdminRoleRepo == nil || l.svcCtx.AdminPermissionRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	if _, err := l.svcCtx.AdminRoleRepo.FindByID(l.ctx, in.RoleId); err != nil {
		return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "NOT_FOUND", "role not found", nil)
	}

	permIDs := uniqueInt64s(in.PermissionIds)
	if len(permIDs) == 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid permission_ids", map[string]string{"permission_ids": "invalid"})
	}

	perms, err := l.svcCtx.AdminPermissionRepo.FindByIDs(l.ctx, permIDs)
	if err != nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "check permissions failed", nil)
	}
	if len(perms) != len(permIDs) {
		return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "NOT_FOUND", "some permissions not found", nil)
	}

	replace := true
	if in.Replace != nil {
		replace = in.Replace.Value
	}

	now := time.Now()
	if err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		if replace {
			if err := tx.WithContext(l.ctx).Where("role_id = ?", in.RoleId).Delete(&model.AdminRolePermissionModel{}).Error; err != nil {
				return errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "cleanup role permissions failed", nil)
			}
		}

		rows := make([]*model.AdminRolePermissionModel, 0, len(permIDs))
		for _, pid := range permIDs {
			pid := pid
			rows = append(rows, &model.AdminRolePermissionModel{
				RoleID:       in.RoleId,
				PermissionID: pid,
				CreatedAt:    &now,
			})
		}
		if err := tx.WithContext(l.ctx).
			Clauses(clause.OnConflict{DoNothing: true}).
			CreateInBatches(rows, 200).Error; err != nil {
			return errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "assign role permissions failed", nil)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return &pb.AssignPermissionsToRoleResponse{
		Success: true,
		Message: "ok",
		Data: &pb.AssignPermissionsToRoleData{
			RoleId:        in.RoleId,
			AssignedCount: int32(len(permIDs)),
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
