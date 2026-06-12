package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/chainsync/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type StartSyncLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewStartSyncLogic(ctx context.Context, svcCtx *svc.ServiceContext) *StartSyncLogic {
	return &StartSyncLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ==================== 区块链状态管理 ====================
func (l *StartSyncLogic) StartSync(in *pb.StartSyncReq) (*pb.StartSyncResp, error) {
	// todo: add your logic here and delete this line

	return &pb.StartSyncResp{}, nil
}
