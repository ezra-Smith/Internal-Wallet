package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/notification/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListTemplatesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListTemplatesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListTemplatesLogic {
	return &ListTemplatesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ==================== 模板管理（Admin）====================
func (l *ListTemplatesLogic) ListTemplates(in *pb.ListTemplatesRequest) (*pb.ListTemplatesResponse, error) {
	// todo: add your logic here and delete this line

	return &pb.ListTemplatesResponse{}, nil
}
