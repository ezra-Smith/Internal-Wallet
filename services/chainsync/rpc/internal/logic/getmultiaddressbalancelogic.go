package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/chainsync/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetMultiAddressBalanceLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetMultiAddressBalanceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMultiAddressBalanceLogic {
	return &GetMultiAddressBalanceLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 获取多地址余额
func (l *GetMultiAddressBalanceLogic) GetMultiAddressBalance(in *pb.GetMultiAddressBalanceReq) (*pb.GetMultiAddressBalanceResp, error) {
	// todo: add your logic here and delete this line

	return &pb.GetMultiAddressBalanceResp{}, nil
}
