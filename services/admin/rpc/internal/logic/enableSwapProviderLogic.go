package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type EnableSwapProviderLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewEnableSwapProviderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EnableSwapProviderLogic {
	return &EnableSwapProviderLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *EnableSwapProviderLogic) EnableSwapProvider(in *pb.AdminEnableSwapProviderRequest) (*pb.AdminEnableSwapProviderResponse, error) {
	if l.svcCtx.SwapRpc == nil {
		l.Logger.Errorf("Swap service client is not available")
		return &pb.AdminEnableSwapProviderResponse{
			Success: false,
			Message: "Swap服务未配置或未启动",
		}, nil
	}

	// 调用 Swap 服务
	swapReq := &pb.SwapEnableProviderRequest{
		Id: in.Id,
	}

	resp, err := l.svcCtx.SwapRpc.EnableSwapProvider(l.ctx, swapReq)
	if err != nil {
		l.Logger.Errorf("Failed to call swap.EnableSwapProvider: %v", err)
		return &pb.AdminEnableSwapProviderResponse{
			Success: false,
			Message: "调用Swap服务失败",
		}, nil
	}

	return &pb.AdminEnableSwapProviderResponse{
		Success: resp.Success,
		Message: resp.Message,
	}, nil
}
