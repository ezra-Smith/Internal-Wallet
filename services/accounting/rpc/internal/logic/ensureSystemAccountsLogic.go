package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/accounting/rpc/internal/engine"
	"internalwallet/services/accounting/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type EnsureSystemAccountsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewEnsureSystemAccountsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EnsureSystemAccountsLogic {
	return &EnsureSystemAccountsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *EnsureSystemAccountsLogic) EnsureSystemAccounts(in *pb.EnsureSystemAccountsRequest) (*pb.EnsureSystemAccountsResponse, error) {
	if err := requireDB(l.svcCtx); err != nil {
		return &pb.EnsureSystemAccountsResponse{Success: false, Message: err.Error()}, nil
	}
	chainCode := ""
	if in != nil {
		chainCode = in.ChainCode
	}
	e := engine.New(l.svcCtx.DB)
	if err := e.EnsureSystemAccounts(l.ctx, chainCode); err != nil {
		l.Logger.Errorf("EnsureSystemAccounts failed: %v", err)
		return &pb.EnsureSystemAccountsResponse{Success: false, Message: err.Error()}, nil
	}
	return &pb.EnsureSystemAccountsResponse{Success: true, Message: "ok"}, nil
}
