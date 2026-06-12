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

type DeletePermissionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeletePermissionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeletePermissionLogic {
	return &DeletePermissionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DeletePermissionLogic) DeletePermission(in *pb.DeletePermissionRequest) (*pb.DeletePermissionResponse, error) {
	if in == nil || in.Id <= 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid id", map[string]string{"id": "required"})
	}
	if l.svcCtx.DB == nil || l.svcCtx.AdminPermissionRepo == nil || l.svcCtx.AdminRolePermissionRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	_, err := l.svcCtx.AdminPermissionRepo.FindByID(l.ctx, in.Id)
	if err != nil {
		return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "NOT_FOUND", "permission not found", nil)
	}

	cnt, err := l.svcCtx.AdminRolePermissionRepo.CountByPermissionID(l.ctx, in.Id)
	if err != nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "check role assignments failed", nil)
	}
	if cnt > 0 && !in.Force {
		return nil, errx.New(codes.FailedPrecondition, 400, errx.CodeInvalidParam, "CANNOT_DELETE", "Permission is assigned to roles, cannot delete", nil)
	}

	now := time.Now()
	if err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		permRepo := l.svcCtx.AdminPermissionRepo.WithTx(tx)
		rpRepo := l.svcCtx.AdminRolePermissionRepo.WithTx(tx)
		if cnt > 0 && in.Force {
			if err := rpRepo.DeleteByPermissionID(l.ctx, in.Id); err != nil {
				return errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "cleanup role-permission failed", nil)
			}
		}
		if err := permRepo.SoftDelete(l.ctx, in.Id, now); err != nil {
			return errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "delete permission failed", nil)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return &pb.DeletePermissionResponse{
		Success:   true,
		Message:   "ok",
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
