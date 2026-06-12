package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/chainsync/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetProviderStatusLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetProviderStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetProviderStatusLogic {
	return &GetProviderStatusLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ==================== 服务商管理 ====================
func (l *GetProviderStatusLogic) GetProviderStatus(in *pb.GetProviderStatusReq) (*pb.GetProviderStatusResp, error) {
	// todo: add your logic here and delete this line

	return &pb.GetProviderStatusResp{}, nil
}
