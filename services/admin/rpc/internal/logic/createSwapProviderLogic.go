package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateSwapProviderLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateSwapProviderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateSwapProviderLogic {
	return &CreateSwapProviderLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateSwapProviderLogic) CreateSwapProvider(in *pb.AdminCreateSwapProviderRequest) (*pb.AdminCreateSwapProviderResponse, error) {
	if l.svcCtx.SwapRpc == nil {
		l.Logger.Errorf("Swap service client is not available")
		return &pb.AdminCreateSwapProviderResponse{
			Success: false,
			Message: "Swap服务未配置或未启动",
		}, nil
	}

	// 调用 Swap 服务
	swapReq := &pb.SwapCreateProviderRequest{
		ProviderCode:     in.ProviderCode,
		ProviderName:     in.ProviderName,
		Description:      in.Description,
		LogoUrl:          in.LogoUrl,
		MinSwapAmountUsd: in.MinSwapAmountUsd,
		MaxSwapAmountUsd: in.MaxSwapAmountUsd,
		DefaultSlippage:  in.DefaultSlippage,
		MaxSlippage:      in.MaxSlippage,
		FeeRate:          in.FeeRate,
		ConfigJson:       in.ConfigJson,
	}

	resp, err := l.svcCtx.SwapRpc.CreateSwapProvider(l.ctx, swapReq)
	if err != nil {
		l.Logger.Errorf("Failed to call swap.CreateSwapProvider: %v", err)
		return &pb.AdminCreateSwapProviderResponse{
			Success: false,
			Message: "调用Swap服务失败",
		}, nil
	}

	return &pb.AdminCreateSwapProviderResponse{
		Success: resp.Success,
		Message: resp.Message,
		Id:      resp.Id,
	}, nil
}
