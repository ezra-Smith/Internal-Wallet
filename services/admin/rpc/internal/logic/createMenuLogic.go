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

type CreateMenuLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateMenuLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateMenuLogic {
	return &CreateMenuLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ==================== RBAC v2 (Menus / Permissions / Roles) ====================
func (l *CreateMenuLogic) CreateMenu(in *pb.CreateMenuRequest) (*pb.CreateMenuResponse, error) {
	if in == nil || strings.TrimSpace(in.Name) == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid params", map[string]string{
			"name": "required",
		})
	}
	if l.svcCtx.DB == nil || l.svcCtx.AdminMenuRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	name := strings.TrimSpace(in.Name)
	pid := in.Pid
	if pid < 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid pid", map[string]string{"pid": "invalid"})
	}

	level := 1
	var tree *string
	if pid > 0 {
		parent, err := l.svcCtx.AdminMenuRepo.FindByID(l.ctx, pid)
		if err != nil || parent == nil {
			return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "NOT_FOUND", "parent menu not found", nil)
		}
		level = parent.Level + 1
		tree = buildChildTree(parent.Tree, parent.ID)
	}

	now := time.Now()
	status := in.Status
	if status == 0 {
		status = 1
	}
	m := &model.AdminMenuModel{
		Pid:                pid,
		Level:              level,
		Tree:               tree,
		Name:               name,
		Path:               strPtrOrNil(in.Path),
		Icon:               strPtrOrNil(in.Icon),
		HideInMenu:         in.HideInMenu,
		HideChildrenInMenu: in.HideChildrenInMenu,
		Sort:               int(in.Sort),
		Remark:             strPtrOrNil(in.Remark),
		Status:             status,
		UpdatedAt:          &now,
		CreatedAt:          &now,
		Target:             strPtrOrNil(in.Target),
		Access:             strPtrOrNil(in.Access),
		Key:                strPtrOrNil(in.Key),
	}

	if err := l.svcCtx.AdminMenuRepo.Create(l.ctx, m); err != nil {
		l.Logger.Errorf("create menu failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "create menu failed", nil)
	}

	return &pb.CreateMenuResponse{
		Success: true,
		Message: "ok",
		Data: &pb.CreateMenuData{
			Menu: toPBAdminMenu(m),
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
