package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/chainsync/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetBlockTransactionsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetBlockTransactionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetBlockTransactionsLogic {
	return &GetBlockTransactionsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 查询特定区块的交易
func (l *GetBlockTransactionsLogic) GetBlockTransactions(in *pb.GetBlockTransactionsReq) (*pb.GetBlockTransactionsResp, error) {
	// todo: add your logic here and delete this line

	return &pb.GetBlockTransactionsResp{}, nil
}
