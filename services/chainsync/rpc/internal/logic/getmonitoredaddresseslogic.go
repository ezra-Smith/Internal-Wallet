package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/chainsync/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetMonitoredAddressesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetMonitoredAddressesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMonitoredAddressesLogic {
	return &GetMonitoredAddressesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 获取监控地址列表
func (l *GetMonitoredAddressesLogic) GetMonitoredAddresses(in *pb.GetMonitoredAddressesReq) (*pb.GetMonitoredAddressesResp, error) {
	if l.svcCtx == nil || l.svcCtx.AddressMonitor == nil {
		return &pb.GetMonitoredAddressesResp{
			Success:     true,
			Monitors:    nil,
			Total:       0,
			ActiveCount: 0,
		}, nil
	}

	return l.svcCtx.AddressMonitor.GetMonitors(in)
}
