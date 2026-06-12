package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/rbac"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetRbacPermissionMapLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetRbacPermissionMapLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetRbacPermissionMapLogic {
	return &GetRbacPermissionMapLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetRbacPermissionMapLogic) GetRbacPermissionMap(
	in *pb.GetRbacPermissionMapRequest,
) (*pb.GetRbacPermissionMapResponse, error) {
	_ = in
	return &pb.GetRbacPermissionMapResponse{
		Success: true,
		Message: "ok",
		Data: &pb.GetRbacPermissionMapData{
			RpcMethodPermissions: rbac.CopyRbacPermissionMap(),
			Version:              rbac.PermissionMapVersion,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
