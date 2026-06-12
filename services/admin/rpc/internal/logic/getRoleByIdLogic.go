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

type GetRoleByIdLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetRoleByIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetRoleByIdLogic {
	return &GetRoleByIdLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetRoleByIdLogic) GetRoleById(in *pb.GetRoleByIdRequest) (*pb.GetRoleByIdResponse, error) {
	if in == nil || in.Id <= 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid id", map[string]string{"id": "required"})
	}
	if l.svcCtx.AdminRoleRepo == nil || l.svcCtx.AdminRBACRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "service not ready", nil)
	}

	role, err := l.svcCtx.AdminRoleRepo.FindByID(l.ctx, in.Id)
	if err != nil || role == nil {
		return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "NOT_FOUND", "role not found", nil)
	}

	out := toPBAdminRole(role)
	if in.IncludeMenus {
		menus, err := l.svcCtx.AdminRBACRepo.GetRoleMenus(l.ctx, in.Id)
		if err != nil {
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "query role menus failed", nil)
		}
		pbMenus := make([]*pb.AdminMenu, 0, len(menus))
		for _, m := range menus {
			pbMenus = append(pbMenus, toPBAdminMenu(m))
		}
		out.Menus = pbMenus
	}
	if in.IncludePermissions {
		perms, err := l.svcCtx.AdminRBACRepo.GetRolePermissions(l.ctx, in.Id)
		if err != nil {
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "query role permissions failed", nil)
		}
		pbPerms := make([]*pb.AdminPermission, 0, len(perms))
		for _, p := range perms {
			pbPerms = append(pbPerms, toPBAdminPermission(p))
		}
		out.Permissions = pbPerms
	}

	return &pb.GetRoleByIdResponse{
		Success: true,
		Message: "ok",
		Data: &pb.GetRoleByIdData{
			Role: out,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
