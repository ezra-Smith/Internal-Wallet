package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/notification/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetUserDevicesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetUserDevicesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserDevicesLogic {
	return &GetUserDevicesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 获取用户设备列表
func (l *GetUserDevicesLogic) GetUserDevices(in *pb.GetUserDevicesRequest) (*pb.GetUserDevicesResponse, error) {
	// todo: add your logic here and delete this line

	return &pb.GetUserDevicesResponse{}, nil
}
