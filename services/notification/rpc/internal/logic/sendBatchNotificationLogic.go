package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/notification/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type SendBatchNotificationLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSendBatchNotificationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SendBatchNotificationLogic {
	return &SendBatchNotificationLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 发送批量推送
func (l *SendBatchNotificationLogic) SendBatchNotification(in *pb.SendBatchNotificationRequest) (*pb.SendBatchNotificationResponse, error) {
	// todo: add your logic here and delete this line

	return &pb.SendBatchNotificationResponse{}, nil
}
