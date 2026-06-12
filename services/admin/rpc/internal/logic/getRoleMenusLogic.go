package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type GetRoleMenusLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetRoleMenusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetRoleMenusLogic {
	return &GetRoleMenusLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetRoleMenusLogic) GetRoleMenus(in *pb.GetRoleMenusRequest) (*pb.GetRoleMenusResponse, error) {
	if in == nil || in.RoleId <= 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid role_id", map[string]string{"role_id": "required"})
	}
	if l.svcCtx.AdminRoleRepo == nil || l.svcCtx.AdminRBACRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "service not ready", nil)
	}

	if _, err := l.svcCtx.AdminRoleRepo.FindByID(l.ctx, in.RoleId); err != nil {
		return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "NOT_FOUND", "role not found", nil)
	}

	menus, err := l.svcCtx.AdminRBACRepo.GetRoleMenus(l.ctx, in.RoleId)
	if err != nil {
		l.Logger.Errorf("get role menus failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "query failed", nil)
	}
	out := make([]*pb.AdminMenu, 0, len(menus))
	for _, m := range menus {
		out = append(out, toPBAdminMenu(m))
	}

	return &pb.GetRoleMenusResponse{
		Success: true,
		Message: "ok",
		Data: &pb.GetRoleMenusData{
			List: out,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
