package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/notification/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateUserSettingsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateUserSettingsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateUserSettingsLogic {
	return &UpdateUserSettingsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 更新用户推送设置
func (l *UpdateUserSettingsLogic) UpdateUserSettings(in *pb.UpdateUserSettingsRequest) (*pb.UpdateUserSettingsResponse, error) {
	// todo: add your logic here and delete this line

	return &pb.UpdateUserSettingsResponse{}, nil
}
