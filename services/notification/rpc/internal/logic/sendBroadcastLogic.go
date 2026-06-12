package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/notification/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type SendBroadcastLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSendBroadcastLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SendBroadcastLogic {
	return &SendBroadcastLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 发送广播推送（所有用户）
func (l *SendBroadcastLogic) SendBroadcast(in *pb.SendBroadcastRequest) (*pb.SendBroadcastResponse, error) {
	// todo: add your logic here and delete this line

	return &pb.SendBroadcastResponse{}, nil
}
