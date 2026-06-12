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

type ListRolesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListRolesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListRolesLogic {
	return &ListRolesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListRolesLogic) ListRoles(in *pb.ListRolesRequest) (*pb.ListRolesResponse, error) {
	if l.svcCtx.AdminRoleRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "service not ready", nil)
	}
	if in == nil {
		in = &pb.ListRolesRequest{}
	}

	var status *int32
	if in.Status != nil {
		v := in.Status.Value
		status = &v
	}

	list, total, err := l.svcCtx.AdminRoleRepo.List(l.ctx, in.Page, in.PageSize, status)
	if err != nil {
		l.Logger.Errorf("list roles failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "list roles failed", nil)
	}

	out := make([]*pb.AdminRole, 0, len(list))
	for _, r := range list {
		out = append(out, toPBAdminRole(r))
	}

	return &pb.ListRolesResponse{
		Success: true,
		Message: "ok",
		Data: &pb.ListRolesData{
			List: out,
		},
		Pagination: calcPagination(in.Page, in.PageSize, total),
		RequestId:  resp.RequestID(l.ctx),
		Timestamp:  resp.Timestamp(),
	}, nil
}
