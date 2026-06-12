package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteSwapConfigLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteSwapConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteSwapConfigLogic {
	return &DeleteSwapConfigLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DeleteSwapConfigLogic) DeleteSwapConfig(in *pb.AdminDeleteSwapConfigRequest) (*pb.AdminDeleteSwapConfigResponse, error) {
	if l.svcCtx.SwapRpc == nil {
		l.Logger.Errorf("Swap service client is not available")
		return &pb.AdminDeleteSwapConfigResponse{
			Success: false,
			Message: "Swap服务未配置或未启动",
		}, nil
	}

	resp, err := l.svcCtx.SwapRpc.DeleteSwapConfig(l.ctx, &pb.SwapDeleteSwapConfigRequest{
		Id: in.Id,
	})
	if err != nil {
		l.Logger.Errorf("Failed to call swap.DeleteSwapConfig: %v", err)
		return &pb.AdminDeleteSwapConfigResponse{
			Success: false,
			Message: "调用Swap服务失败",
		}, nil
	}

	return &pb.AdminDeleteSwapConfigResponse{
		Success: resp.Success,
		Message: resp.Message,
	}, nil
}
