package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/chainsync/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetSyncStatsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetSyncStatsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSyncStatsLogic {
	return &GetSyncStatsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ==================== 数据统计 ====================
func (l *GetSyncStatsLogic) GetSyncStats(in *pb.GetSyncStatsReq) (*pb.GetSyncStatsResp, error) {
	// todo: add your logic here and delete this line

	return &pb.GetSyncStatsResp{}, nil
}
