package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/chainsync/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetTransactionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetTransactionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetTransactionLogic {
	return &GetTransactionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ==================== 交易监控 ====================
func (l *GetTransactionLogic) GetTransaction(in *pb.GetChainTransactionReq) (*pb.GetChainTransactionResp, error) {
	// todo: add your logic here and delete this line

	return &pb.GetChainTransactionResp{}, nil
}
