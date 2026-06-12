package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/notification/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetUserSettingsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetUserSettingsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserSettingsLogic {
	return &GetUserSettingsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ==================== 用户设置 ====================
func (l *GetUserSettingsLogic) GetUserSettings(in *pb.GetUserSettingsRequest) (*pb.GetUserSettingsResponse, error) {
	// todo: add your logic here and delete this line

	return &pb.GetUserSettingsResponse{}, nil
}
