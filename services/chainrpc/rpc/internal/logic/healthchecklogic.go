package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/chainrpc/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type HealthCheckLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewHealthCheckLogic(ctx context.Context, svcCtx *svc.ServiceContext) *HealthCheckLogic {
	return &HealthCheckLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ==================== 节点管理 ====================
func (l *HealthCheckLogic) HealthCheck(in *pb.HealthCheckReq) (*pb.HealthCheckResp, error) {
	// todo: add your logic here and delete this line

	return &pb.HealthCheckResp{}, nil
}
