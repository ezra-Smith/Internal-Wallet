package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/chainsync/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ResetSyncLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewResetSyncLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResetSyncLogic {
	return &ResetSyncLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 重置同步位置
func (l *ResetSyncLogic) ResetSync(in *pb.ResetSyncReq) (*pb.ResetSyncResp, error) {
	// todo: add your logic here and delete this line

	return &pb.ResetSyncResp{}, nil
}
