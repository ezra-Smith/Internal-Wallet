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

type AssignMenusToRoleLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAssignMenusToRoleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AssignMenusToRoleLogic {
	return &AssignMenusToRoleLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AssignMenusToRoleLogic) AssignMenusToRole(in *pb.AssignMenusToRoleRequest) (*pb.AssignMenusToRoleResponse, error) {
	if in == nil || in.RoleId <= 0 || len(in.MenuIds) == 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid params", map[string]string{
			"role_id":  "required",
			"menu_ids": "required",
		})
	}
	if l.svcCtx.DB == nil || l.svcCtx.AdminRoleRepo == nil || l.svcCtx.AdminMenuRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	if _, err := l.svcCtx.AdminRoleRepo.FindByID(l.ctx, in.RoleId); err != nil {
		return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "NOT_FOUND", "role not found", nil)
	}

	menuIDs := uniqueInt64s(in.MenuIds)
	if len(menuIDs) == 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid menu_ids", map[string]string{"menu_ids": "invalid"})
	}

	menus, err := l.svcCtx.AdminMenuRepo.FindByIDs(l.ctx, menuIDs)
	if err != nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "check menus failed", nil)
	}
	if len(menus) != len(menuIDs) {
		return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "NOT_FOUND", "some menus not found", nil)
	}

	replace := true
	if in.Replace != nil {
		replace = in.Replace.Value
	}

	now := time.Now()
	if err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		if replace {
			if err := tx.WithContext(l.ctx).Where("role_id = ?", in.RoleId).Delete(&model.AdminRoleMenuModel{}).Error; err != nil {
				return errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "cleanup role menus failed", nil)
			}
		}

		rows := make([]*model.AdminRoleMenuModel, 0, len(menuIDs))
		for _, mid := range menuIDs {
			mid := mid
			rows = append(rows, &model.AdminRoleMenuModel{
				RoleID:    in.RoleId,
				MenuID:    mid,
				CreatedAt: &now,
			})
		}
		if err := tx.WithContext(l.ctx).
			Clauses(clause.OnConflict{DoNothing: true}).
			CreateInBatches(rows, 200).Error; err != nil {
			return errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "assign role menus failed", nil)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return &pb.AssignMenusToRoleResponse{
		Success: true,
		Message: "ok",
		Data: &pb.AssignMenusToRoleData{
			RoleId:        in.RoleId,
			AssignedCount: int32(len(menuIDs)),
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
