package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/chainsync/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetSyncStatusLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetSyncStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSyncStatusLogic {
	return &GetSyncStatusLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 获取同步状态
func (l *GetSyncStatusLogic) GetSyncStatus(in *pb.GetSyncStatusReq) (*pb.GetSyncStatusResp, error) {
	// todo: add your logic here and delete this line

	return &pb.GetSyncStatusResp{}, nil
}
