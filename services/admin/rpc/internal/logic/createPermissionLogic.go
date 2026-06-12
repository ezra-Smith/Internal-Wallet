package logic

import (
	"context"
	"strings"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type CreatePermissionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreatePermissionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreatePermissionLogic {
	return &CreatePermissionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// -------- Permissions --------
func (l *CreatePermissionLogic) CreatePermission(in *pb.CreatePermissionRequest) (*pb.CreatePermissionResponse, error) {
	if in == nil || strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.Code) == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid params", map[string]string{
			"name": "required",
			"code": "required",
		})
	}
	if l.svcCtx.DB == nil || l.svcCtx.AdminPermissionRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	name := strings.TrimSpace(in.Name)
	code := strings.TrimSpace(in.Code)

	exists, err := l.svcCtx.AdminPermissionRepo.ExistsByCode(l.ctx, code, 0)
	if err != nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "check code failed", nil)
	}
	if exists {
		return nil, errx.New(codes.AlreadyExists, 409, errx.CodeConflict, "CONFLICT", "permission code already exists", map[string]string{"code": "exists"})
	}

	var menuID *int64
	if in.MenuId != nil {
		if in.MenuId.Value > 0 {
			if l.svcCtx.AdminMenuRepo == nil {
				return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "service not ready", nil)
			}
			if _, err := l.svcCtx.AdminMenuRepo.FindByID(l.ctx, in.MenuId.Value); err != nil {
				return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "NOT_FOUND", "menu not found", nil)
			}
			menuID = int64Ptr(in.MenuId.Value)
		} else {
			menuID = nil
		}
	}

	status := in.Status
	if status == 0 {
		status = 1
	}
	now := time.Now()
	m := &model.AdminPermissionModel{
		Name:        name,
		Code:        code,
		Description: strPtrOrNil(in.Description),
		MenuID:      menuID,
		Status:      status,
		CreatedAt:   &now,
		UpdatedAt:   &now,
	}
	if err := l.svcCtx.AdminPermissionRepo.Create(l.ctx, m); err != nil {
		l.Logger.Errorf("create permission failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "create permission failed", nil)
	}

	return &pb.CreatePermissionResponse{
		Success: true,
		Message: "ok",
		Data: &pb.CreatePermissionData{
			Permission: toPBAdminPermission(m),
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
