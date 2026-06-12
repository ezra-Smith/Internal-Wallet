package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type DisableSwapProviderLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDisableSwapProviderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DisableSwapProviderLogic {
	return &DisableSwapProviderLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DisableSwapProviderLogic) DisableSwapProvider(in *pb.AdminDisableSwapProviderRequest) (*pb.AdminDisableSwapProviderResponse, error) {
	if l.svcCtx.SwapRpc == nil {
		l.Logger.Errorf("Swap service client is not available")
		return &pb.AdminDisableSwapProviderResponse{
			Success: false,
			Message: "Swap服务未配置或未启动",
		}, nil
	}

	// 调用 Swap 服务
	swapReq := &pb.SwapDisableProviderRequest{
		Id: in.Id,
	}

	resp, err := l.svcCtx.SwapRpc.DisableSwapProvider(l.ctx, swapReq)
	if err != nil {
		l.Logger.Errorf("Failed to call swap.DisableSwapProvider: %v", err)
		return &pb.AdminDisableSwapProviderResponse{
			Success: false,
			Message: "调用Swap服务失败",
		}, nil
	}

	return &pb.AdminDisableSwapProviderResponse{
		Success: resp.Success,
		Message: resp.Message,
	}, nil
}
