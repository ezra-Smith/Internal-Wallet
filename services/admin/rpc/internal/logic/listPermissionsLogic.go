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

type ListPermissionsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListPermissionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPermissionsLogic {
	return &ListPermissionsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListPermissionsLogic) ListPermissions(in *pb.ListPermissionsRequest) (*pb.ListPermissionsResponse, error) {
	if l.svcCtx.AdminPermissionRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "service not ready", nil)
	}
	if in == nil {
		in = &pb.ListPermissionsRequest{}
	}

	var menuID *int64
	if in.MenuId != nil {
		v := in.MenuId.Value
		menuID = &v
	}
	var status *int32
	if in.Status != nil {
		v := in.Status.Value
		status = &v
	}

	list, total, err := l.svcCtx.AdminPermissionRepo.List(l.ctx, in.Page, in.PageSize, menuID, status, in.Name, in.Code, in.Description)
	if err != nil {
		l.Logger.Errorf("list permissions failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "list permissions failed", nil)
	}

	out := make([]*pb.AdminPermission, 0, len(list))
	for _, p := range list {
		out = append(out, toPBAdminPermission(p))
	}

	return &pb.ListPermissionsResponse{
		Success: true,
		Message: "ok",
		Data: &pb.ListPermissionsData{
			List: out,
		},
		Pagination: calcPagination(in.Page, in.PageSize, total),
		RequestId:  resp.RequestID(l.ctx),
		Timestamp:  resp.Timestamp(),
	}, nil
}
