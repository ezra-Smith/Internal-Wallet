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

type ListMenusLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListMenusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListMenusLogic {
	return &ListMenusLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListMenusLogic) ListMenus(in *pb.ListMenusRequest) (*pb.ListMenusResponse, error) {
	if l.svcCtx.AdminMenuRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "service not ready", nil)
	}
	if in == nil {
		in = &pb.ListMenusRequest{}
	}

	var status *int32
	if in.Status != nil {
		v := in.Status.Value
		status = &v
	}
	var pid *int64
	if in.Pid != nil {
		v := in.Pid.Value
		pid = &v
	}
	var hide *bool
	if in.HideInMenu != nil {
		v := in.HideInMenu.Value
		hide = &v
	}

	list, total, err := l.svcCtx.AdminMenuRepo.List(l.ctx, in.Page, in.PageSize, status, pid, hide)
	if err != nil {
		l.Logger.Errorf("list menus failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "list menus failed", nil)
	}

	out := make([]*pb.AdminMenu, 0, len(list))
	for _, m := range list {
		out = append(out, toPBAdminMenu(m))
	}

	return &pb.ListMenusResponse{
		Success: true,
		Message: "ok",
		Data: &pb.ListMenusData{
			List: out,
		},
		Pagination: calcPagination(in.Page, in.PageSize, total),
		RequestId:  resp.RequestID(l.ctx),
		Timestamp:  resp.Timestamp(),
	}, nil
}
