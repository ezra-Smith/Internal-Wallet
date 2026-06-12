package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/notification/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type MarkNotificationReadLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewMarkNotificationReadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MarkNotificationReadLogic {
	return &MarkNotificationReadLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 标记推送为已读
func (l *MarkNotificationReadLogic) MarkNotificationRead(in *pb.MarkNotificationReadRequest) (*pb.MarkNotificationReadResponse, error) {
	// todo: add your logic here and delete this line

	return &pb.MarkNotificationReadResponse{}, nil
}
