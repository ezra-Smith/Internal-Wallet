package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/chainrpc/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetNodeStatusLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetNodeStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNodeStatusLogic {
	return &GetNodeStatusLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 获取节点状态
func (l *GetNodeStatusLogic) GetNodeStatus(in *pb.GetNodeStatusReq) (*pb.GetNodeStatusResp, error) {
	// todo: add your logic here and delete this line

	return &pb.GetNodeStatusResp{}, nil
}
