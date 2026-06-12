package logic

import (
	"context"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"gorm.io/gorm"
)

type DeleteRoleLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteRoleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteRoleLogic {
	return &DeleteRoleLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DeleteRoleLogic) DeleteRole(in *pb.DeleteRoleRequest) (*pb.DeleteRoleResponse, error) {
	if in == nil || in.Id <= 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid id", map[string]string{"id": "required"})
	}
	if l.svcCtx.DB == nil || l.svcCtx.AdminRoleRepo == nil || l.svcCtx.AdminUserRoleRepo == nil || l.svcCtx.AdminRoleMenuRepo == nil || l.svcCtx.AdminRolePermissionRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	_, err := l.svcCtx.AdminRoleRepo.FindByID(l.ctx, in.Id)
	if err != nil {
		return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "NOT_FOUND", "role not found", nil)
	}

	cnt, err := l.svcCtx.AdminUserRoleRepo.CountByRoleID(l.ctx, in.Id)
	if err != nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "check user assignments failed", nil)
	}
	if cnt > 0 && !in.Force {
		return nil, errx.New(codes.FailedPrecondition, 400, errx.CodeInvalidParam, "CANNOT_DELETE", "Role is assigned to users, cannot delete", nil)
	}

	now := time.Now()
	if err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		roleRepo := l.svcCtx.AdminRoleRepo.WithTx(tx)
		userRoleRepo := l.svcCtx.AdminUserRoleRepo.WithTx(tx)
		roleMenuRepo := l.svcCtx.AdminRoleMenuRepo.WithTx(tx)
		rolePermRepo := l.svcCtx.AdminRolePermissionRepo.WithTx(tx)

		// cleanup junctions (soft delete does not trigger FK cascades)
		if in.Force {
			if err := userRoleRepo.DeleteByRoleID(l.ctx, in.Id); err != nil {
				return errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "cleanup user-role failed", nil)
			}
		}
		_ = roleMenuRepo.DeleteByRoleID(l.ctx, in.Id)
		_ = rolePermRepo.DeleteByRoleID(l.ctx, in.Id)

		if err := roleRepo.SoftDelete(l.ctx, in.Id, now); err != nil {
			return errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "delete role failed", nil)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return &pb.DeleteRoleResponse{
		Success:   true,
		Message:   "ok",
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
