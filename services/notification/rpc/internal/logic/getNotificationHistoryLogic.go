package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/notification/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetNotificationHistoryLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetNotificationHistoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNotificationHistoryLogic {
	return &GetNotificationHistoryLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ==================== 推送历史 ====================
func (l *GetNotificationHistoryLogic) GetNotificationHistory(in *pb.GetNotificationHistoryRequest) (*pb.GetNotificationHistoryResponse, error) {
	// todo: add your logic here and delete this line

	return &pb.GetNotificationHistoryResponse{}, nil
}
