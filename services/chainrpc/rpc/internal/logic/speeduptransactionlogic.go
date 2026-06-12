package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/chainrpc/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type SpeedUpTransactionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSpeedUpTransactionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SpeedUpTransactionLogic {
	return &SpeedUpTransactionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 加速交易（Replace-By-Fee）
func (l *SpeedUpTransactionLogic) SpeedUpTransaction(in *pb.SpeedUpTransactionReq) (*pb.SpeedUpTransactionResp, error) {
	// todo: add your logic here and delete this line

	return &pb.SpeedUpTransactionResp{}, nil
}
