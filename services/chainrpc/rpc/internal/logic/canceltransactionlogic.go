package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/chainrpc/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CancelTransactionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCancelTransactionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CancelTransactionLogic {
	return &CancelTransactionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 取消交易
func (l *CancelTransactionLogic) CancelTransaction(in *pb.CancelTransactionReq) (*pb.CancelTransactionResp, error) {
	// todo: add your logic here and delete this line

	return &pb.CancelTransactionResp{}, nil
}
