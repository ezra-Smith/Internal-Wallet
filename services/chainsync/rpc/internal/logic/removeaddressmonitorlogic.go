package logic

import (
	"context"
	"fmt"

	"internalwallet/proto/pb"
	"internalwallet/services/chainsync/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type RemoveAddressMonitorLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRemoveAddressMonitorLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RemoveAddressMonitorLogic {
	return &RemoveAddressMonitorLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 移除地址监控
func (l *RemoveAddressMonitorLogic) RemoveAddressMonitor(in *pb.RemoveAddressMonitorReq) (*pb.RemoveAddressMonitorResp, error) {
	//验证参数
	if in.Address == "" {
		return &pb.RemoveAddressMonitorResp{
			Success:      false,
			Message:      "参数错误",
			RemovedCount: 0,
		}, fmt.Errorf("address cannot be empty")
	}
	resp, err := l.svcCtx.AddressMonitor.RemoveMonitor(in)
	if err != nil {
		logx.Errorf("Failed to remove address monitor: %v", err)
		return &pb.RemoveAddressMonitorResp{
			Success:      false,
			Message:      "移除监控地址失败",
			RemovedCount: 0,
		}, err
	}
	logx.Infof("✅ Removed address monitor for %s on chain %v, removed count: %d",
		in.Address, in.Chain, resp.RemovedCount)
	return resp, nil
}
