package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/accounting/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetUserTransactionRecordDetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetUserTransactionRecordDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserTransactionRecordDetailLogic {
	return &GetUserTransactionRecordDetailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetUserTransactionRecordDetailLogic) GetUserTransactionRecordDetail(in *pb.GetUserTransactionRecordDetailRequest) (*pb.GetUserTransactionRecordDetailResponse, error) {
	if err := requireDB(l.svcCtx); err != nil {
		return &pb.GetUserTransactionRecordDetailResponse{Success: false, Message: err.Error()}, nil
	}
	if l.svcCtx.UserTxRepo == nil {
		return &pb.GetUserTransactionRecordDetailResponse{Success: false, Message: "repository not initialized"}, nil
	}
	if in == nil || in.Id <= 0 {
		return &pb.GetUserTransactionRecordDetailResponse{Success: false, Message: "invalid params"}, nil
	}

	m, err := l.svcCtx.UserTxRepo.FindByID(l.ctx, in.Id)
	if err != nil || m == nil {
		return &pb.GetUserTransactionRecordDetailResponse{Success: false, Message: "not found"}, nil
	}
	return &pb.GetUserTransactionRecordDetailResponse{Success: true, Message: "ok", Item: toPBUserTxRecordItem(m)}, nil
}
