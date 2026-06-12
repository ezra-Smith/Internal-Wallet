package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/chainsync/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type StopSyncLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewStopSyncLogic(ctx context.Context, svcCtx *svc.ServiceContext) *StopSyncLogic {
	return &StopSyncLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 停止区块链同步
func (l *StopSyncLogic) StopSync(in *pb.StopSyncReq) (*pb.StopSyncResp, error) {
	// todo: add your logic here and delete this line

	return &pb.StopSyncResp{}, nil
}
