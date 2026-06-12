package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/chainsync/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type TestProviderLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewTestProviderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TestProviderLogic {
	return &TestProviderLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 测试服务商连接
func (l *TestProviderLogic) TestProvider(in *pb.TestProviderReq) (*pb.TestProviderResp, error) {
	// todo: add your logic here and delete this line

	return &pb.TestProviderResp{}, nil
}
