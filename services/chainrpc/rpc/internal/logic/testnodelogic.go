package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/chainrpc/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type TestNodeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewTestNodeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TestNodeLogic {
	return &TestNodeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 测试节点连接
func (l *TestNodeLogic) TestNode(in *pb.TestNodeReq) (*pb.TestNodeResp, error) {
	// todo: add your logic here and delete this line

	return &pb.TestNodeResp{}, nil
}
