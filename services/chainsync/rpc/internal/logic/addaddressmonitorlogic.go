package logic

import (
	"context"
	"fmt"

	"internalwallet/proto/pb"
	"internalwallet/services/chainsync/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type AddAddressMonitorLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAddAddressMonitorLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddAddressMonitorLogic {
	return &AddAddressMonitorLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ==================== 地址监控 ====================
func (l *AddAddressMonitorLogic) AddAddressMonitor(in *pb.AddAddressMonitorReq) (*pb.AddAddressMonitorResp, error) {
	// 验证请求参数
	if in.Address == "" {
		return &pb.AddAddressMonitorResp{
			Success:   false,
			Message:   "地址不能为空",
			MonitorId: "",
		}, fmt.Errorf("address cannot be empty")
	}

	// 调用地址监控服务
	resp, err := l.svcCtx.AddressMonitor.AddMonitor(in)
	if err != nil {
		logx.Errorf("Failed to add address monitor: %v", err)
		return &pb.AddAddressMonitorResp{
			Success:   false,
			Message:   fmt.Sprintf("添加地址监控失败: %v", err),
			MonitorId: "",
		}, err
	}

	logx.Infof("✅ Added address monitor for %s on chain %v, monitor ID: %s",
		in.Address, in.Chain, resp.MonitorId)

	return resp, nil
}
