package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/signer/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GenerateAddressBatchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGenerateAddressBatchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GenerateAddressBatchLogic {
	return &GenerateAddressBatchLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 批量生成地址
func (l *GenerateAddressBatchLogic) GenerateAddressBatch(in *pb.GenerateAddressBatchRequest) (*pb.GenerateAddressBatchResponse, error) {
	// todo: add your logic here and delete this line

	return &pb.GenerateAddressBatchResponse{}, nil
}
