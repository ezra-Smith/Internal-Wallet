package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/chainrpc/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetMultiBalanceLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetMultiBalanceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMultiBalanceLogic {
	return &GetMultiBalanceLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 查询多地址余额
func (l *GetMultiBalanceLogic) GetMultiBalance(in *pb.GetMultiBalanceReq) (*pb.GetMultiBalanceResp, error) {
	// todo: add your logic here and delete this line

	return &pb.GetMultiBalanceResp{}, nil
}
