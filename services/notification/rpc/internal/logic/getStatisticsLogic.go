package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/notification/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetStatisticsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetStatisticsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetStatisticsLogic {
	return &GetStatisticsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ==================== 统计分析 ====================
func (l *GetStatisticsLogic) GetStatistics(in *pb.GetStatisticsRequest) (*pb.GetStatisticsResponse, error) {
	// todo: add your logic here and delete this line

	return &pb.GetStatisticsResponse{}, nil
}
