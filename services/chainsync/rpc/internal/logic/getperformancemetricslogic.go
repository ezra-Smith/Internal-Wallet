package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/chainsync/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetPerformanceMetricsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetPerformanceMetricsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPerformanceMetricsLogic {
	return &GetPerformanceMetricsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 获取性能指标
func (l *GetPerformanceMetricsLogic) GetPerformanceMetrics(in *pb.GetPerformanceMetricsReq) (*pb.GetPerformanceMetricsResp, error) {
	// todo: add your logic here and delete this line

	return &pb.GetPerformanceMetricsResp{}, nil
}
