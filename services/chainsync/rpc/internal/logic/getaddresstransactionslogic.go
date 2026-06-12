package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/chainsync/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetAddressTransactionsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetAddressTransactionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAddressTransactionsLogic {
	return &GetAddressTransactionsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 获取地址交易历史
func (l *GetAddressTransactionsLogic) GetAddressTransactions(in *pb.GetAddressTransactionsReq) (*pb.GetAddressTransactionsResp, error) {
	// todo: add your logic here and delete this line

	return &pb.GetAddressTransactionsResp{}, nil
}
