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

type CreateRoleLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateRoleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateRoleLogic {
	return &CreateRoleLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// -------- Roles --------
func (l *CreateRoleLogic) CreateRole(in *pb.CreateRoleRequest) (*pb.CreateRoleResponse, error) {
	if in == nil || strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.Code) == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid params", map[string]string{
			"name": "required",
			"code": "required",
		})
	}
	if l.svcCtx.DB == nil || l.svcCtx.AdminRoleRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	name := strings.TrimSpace(in.Name)
	code := strings.TrimSpace(in.Code)
	exists, err := l.svcCtx.AdminRoleRepo.ExistsByCode(l.ctx, code, 0)
	if err != nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "check code failed", nil)
	}
	if exists {
		return nil, errx.New(codes.AlreadyExists, 409, errx.CodeConflict, "CONFLICT", "role code already exists", map[string]string{"code": "exists"})
	}

	status := in.Status
	if status == 0 {
		status = 1
	}
	now := time.Now()
	m := &model.AdminRoleModel{
		Name:        name,
		Code:        code,
		Description: strPtrOrNil(in.Description),
		Status:      status,
		CreatedAt:   &now,
		UpdatedAt:   &now,
	}
	if err := l.svcCtx.AdminRoleRepo.Create(l.ctx, m); err != nil {
		l.Logger.Errorf("create role failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "create role failed", nil)
	}

	return &pb.CreateRoleResponse{
		Success: true,
		Message: "ok",
		Data: &pb.CreateRoleData{
			Role: toPBAdminRole(m),
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
