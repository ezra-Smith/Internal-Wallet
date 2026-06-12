package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/notification/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateTemplateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateTemplateLogic {
	return &UpdateTemplateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 更新推送模板
func (l *UpdateTemplateLogic) UpdateTemplate(in *pb.UpdateTemplateRequest) (*pb.UpdateTemplateResponse, error) {
	// todo: add your logic here and delete this line

	return &pb.UpdateTemplateResponse{}, nil
}
