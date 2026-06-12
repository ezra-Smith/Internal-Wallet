package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/chainsync/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetAddressMonitorStatsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetAddressMonitorStatsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAddressMonitorStatsLogic {
	return &GetAddressMonitorStatsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetAddressMonitorStatsLogic) GetAddressMonitorStats(in *pb.GetAddressMonitorStatsReq) (*pb.GetAddressMonitorStatsResp, error) {
	if l.svcCtx == nil || l.svcCtx.AddressMonitor == nil {
		return &pb.GetAddressMonitorStatsResp{
			Success: true,
			Message: "ok",
			Data:    &pb.AddressMonitorStatsData{},
		}, nil
	}

	return l.svcCtx.AddressMonitor.GetAddressMonitorStats(in)
}
