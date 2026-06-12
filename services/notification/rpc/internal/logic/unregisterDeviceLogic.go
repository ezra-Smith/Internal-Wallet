package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/notification/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type UnregisterDeviceLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUnregisterDeviceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UnregisterDeviceLogic {
	return &UnregisterDeviceLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 注销设备（用户登出时调用）
func (l *UnregisterDeviceLogic) UnregisterDevice(in *pb.UnregisterDeviceRequest) (*pb.UnregisterDeviceResponse, error) {
	// todo: add your logic here and delete this line

	return &pb.UnregisterDeviceResponse{}, nil
}
