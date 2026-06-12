package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/chainsync/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type SwitchProviderLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSwitchProviderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SwitchProviderLogic {
	return &SwitchProviderLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 切换服务商
func (l *SwitchProviderLogic) SwitchProvider(in *pb.SwitchProviderReq) (*pb.SwitchProviderResp, error) {
	// todo: add your logic here and delete this line

	return &pb.SwitchProviderResp{}, nil
}
