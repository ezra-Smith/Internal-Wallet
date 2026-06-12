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

type GetPermissionsByMenuIdLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetPermissionsByMenuIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPermissionsByMenuIdLogic {
	return &GetPermissionsByMenuIdLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetPermissionsByMenuIdLogic) GetPermissionsByMenuId(in *pb.GetPermissionsByMenuIdRequest) (*pb.GetPermissionsByMenuIdResponse, error) {
	if in == nil || in.MenuId <= 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid menu_id", map[string]string{"menu_id": "required"})
	}
	if l.svcCtx.AdminPermissionRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "service not ready", nil)
	}

	list, err := l.svcCtx.AdminPermissionRepo.ListActiveByMenuID(l.ctx, in.MenuId)
	if err != nil {
		l.Logger.Errorf("get permissions by menu_id failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "query failed", nil)
	}
	out := make([]*pb.AdminPermission, 0, len(list))
	for _, p := range list {
		out = append(out, toPBAdminPermission(p))
	}

	return &pb.GetPermissionsByMenuIdResponse{
		Success: true,
		Message: "ok",
		Data: &pb.GetPermissionsByMenuIdData{
			List: out,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
