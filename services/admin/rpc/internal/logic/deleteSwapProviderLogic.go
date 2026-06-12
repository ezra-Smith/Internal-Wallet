package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteSwapProviderLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteSwapProviderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteSwapProviderLogic {
	return &DeleteSwapProviderLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DeleteSwapProviderLogic) DeleteSwapProvider(in *pb.AdminDeleteSwapProviderRequest) (*pb.AdminDeleteSwapProviderResponse, error) {
	if l.svcCtx.SwapRpc == nil {
		l.Logger.Errorf("Swap service client is not available")
		return &pb.AdminDeleteSwapProviderResponse{
			Success: false,
			Message: "Swap服务未配置或未启动",
		}, nil
	}

	// 调用 Swap 服务
	swapReq := &pb.SwapDeleteProviderRequest{
		Id: in.Id,
	}

	resp, err := l.svcCtx.SwapRpc.DeleteSwapProvider(l.ctx, swapReq)
	if err != nil {
		l.Logger.Errorf("Failed to call swap.DeleteSwapProvider: %v", err)
		return &pb.AdminDeleteSwapProviderResponse{
			Success: false,
			Message: "调用Swap服务失败",
		}, nil
	}

	return &pb.AdminDeleteSwapProviderResponse{
		Success: resp.Success,
		Message: resp.Message,
	}, nil
}
