package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/chainrpc/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type SwitchNodeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSwitchNodeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SwitchNodeLogic {
	return &SwitchNodeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 切换节点
func (l *SwitchNodeLogic) SwitchNode(in *pb.SwitchNodeReq) (*pb.SwitchNodeResp, error) {
	// todo: add your logic here and delete this line

	return &pb.SwitchNodeResp{}, nil
}
