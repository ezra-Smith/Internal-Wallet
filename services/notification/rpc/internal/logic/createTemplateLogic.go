package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/notification/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateTemplateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateTemplateLogic {
	return &CreateTemplateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 创建推送模板
func (l *CreateTemplateLogic) CreateTemplate(in *pb.CreateTemplateRequest) (*pb.CreateTemplateResponse, error) {
	// todo: add your logic here and delete this line

	return &pb.CreateTemplateResponse{}, nil
}
