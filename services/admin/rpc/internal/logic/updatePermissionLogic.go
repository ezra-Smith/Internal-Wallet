package logic

import (
	"context"
	"strings"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type UpdatePermissionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdatePermissionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdatePermissionLogic {
	return &UpdatePermissionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdatePermissionLogic) UpdatePermission(in *pb.UpdatePermissionRequest) (*pb.UpdatePermissionResponse, error) {
	if in == nil || in.Id <= 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid id", map[string]string{"id": "required"})
	}
	if l.svcCtx.DB == nil || l.svcCtx.AdminPermissionRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	_, err := l.svcCtx.AdminPermissionRepo.FindByID(l.ctx, in.Id)
	if err != nil {
		return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "NOT_FOUND", "permission not found", nil)
	}

	now := time.Now()
	fields := map[string]interface{}{
		"updated_at": &now,
	}

	if in.Name != nil {
		v := strings.TrimSpace(in.Name.Value)
		if v == "" {
			return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid name", map[string]string{"name": "required"})
		}
		fields["name"] = v
	}
	if in.Code != nil {
		v := strings.TrimSpace(in.Code.Value)
		if v == "" {
			return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid code", map[string]string{"code": "required"})
		}
		exists, err := l.svcCtx.AdminPermissionRepo.ExistsByCode(l.ctx, v, in.Id)
		if err != nil {
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "check code failed", nil)
		}
		if exists {
			return nil, errx.New(codes.AlreadyExists, 409, errx.CodeConflict, "CONFLICT", "permission code already exists", map[string]string{"code": "exists"})
		}
		fields["code"] = v
	}
	if in.MenuId != nil {
		if in.MenuId.Value > 0 {
			if l.svcCtx.AdminMenuRepo == nil {
				return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "service not ready", nil)
			}
			if _, err := l.svcCtx.AdminMenuRepo.FindByID(l.ctx, in.MenuId.Value); err != nil {
				return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "NOT_FOUND", "menu not found", nil)
			}
			fields["menu_id"] = in.MenuId.Value
		} else {
			fields["menu_id"] = nil
		}
	}
	if in.Description != nil {
		fields["description"] = strPtrOrNil(in.Description.Value)
	}
	if in.Status != nil {
		fields["status"] = in.Status.Value
	}

	if err := l.svcCtx.AdminPermissionRepo.UpdateFields(l.ctx, in.Id, fields); err != nil {
		l.Logger.Errorf("update permission failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "update permission failed", nil)
	}

	updated, err := l.svcCtx.AdminPermissionRepo.FindByID(l.ctx, in.Id)
	if err != nil || updated == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "permission updated but reload failed", nil)
	}

	return &pb.UpdatePermissionResponse{
		Success: true,
		Message: "ok",
		Data: &pb.UpdatePermissionData{
			Permission: toPBAdminPermission(updated),
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
