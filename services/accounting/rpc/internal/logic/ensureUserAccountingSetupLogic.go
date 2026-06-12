package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/accounting/rpc/internal/engine"
	"internalwallet/services/accounting/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type EnsureUserAccountingSetupLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewEnsureUserAccountingSetupLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EnsureUserAccountingSetupLogic {
	return &EnsureUserAccountingSetupLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ==================== Setup ====================
func (l *EnsureUserAccountingSetupLogic) EnsureUserAccountingSetup(in *pb.EnsureUserAccountingSetupRequest) (*pb.EnsureUserAccountingSetupResponse, error) {
	if err := requireDB(l.svcCtx); err != nil {
		return &pb.EnsureUserAccountingSetupResponse{Success: false, Message: err.Error()}, nil
	}
	if in == nil || in.UserId <= 0 {
		return &pb.EnsureUserAccountingSetupResponse{Success: false, Message: "invalid user_id"}, nil
	}
	e := engine.New(l.svcCtx.DB)
	if err := e.EnsureUserAccountingSetup(l.ctx, in.UserId); err != nil {
		l.Logger.Errorf("EnsureUserAccountingSetup failed: %v", err)
		return &pb.EnsureUserAccountingSetupResponse{Success: false, Message: err.Error()}, nil
	}
	return &pb.EnsureUserAccountingSetupResponse{Success: true, Message: "ok"}, nil
}
