package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/notification/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetNotificationDetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetNotificationDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNotificationDetailLogic {
	return &GetNotificationDetailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 获取推送详情
func (l *GetNotificationDetailLogic) GetNotificationDetail(in *pb.GetNotificationDetailRequest) (*pb.GetNotificationDetailResponse, error) {
	// todo: add your logic here and delete this line

	return &pb.GetNotificationDetailResponse{}, nil
}
