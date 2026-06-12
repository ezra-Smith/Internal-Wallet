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

type GetMenuTreeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetMenuTreeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMenuTreeLogic {
	return &GetMenuTreeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetMenuTreeLogic) GetMenuTree(in *pb.GetMenuTreeRequest) (*pb.GetMenuTreeResponse, error) {
	_ = in
	if l.svcCtx.AdminMenuRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "service not ready", nil)
	}

	rootPid := int64(0)
	if in != nil && in.RootPid != nil {
		rootPid = in.RootPid.Value
	}

	menus, err := l.svcCtx.AdminMenuRepo.ListAllActiveForTree(l.ctx)
	if err != nil {
		l.Logger.Errorf("get menu tree failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "get menu tree failed", nil)
	}
	tree := buildMenuTreeFromModels(menus, rootPid, false, false)

	return &pb.GetMenuTreeResponse{
		Success: true,
		Message: "ok",
		Data: &pb.GetMenuTreeData{
			List: tree,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
