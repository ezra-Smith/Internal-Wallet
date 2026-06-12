package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/chainrpc/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetTransactionReceiptLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetTransactionReceiptLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetTransactionReceiptLogic {
	return &GetTransactionReceiptLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 查询交易收据
func (l *GetTransactionReceiptLogic) GetTransactionReceipt(in *pb.GetTransactionReceiptReq) (*pb.GetTransactionReceiptResp, error) {
	// todo: add your logic here and delete this line

	return &pb.GetTransactionReceiptResp{}, nil
}
