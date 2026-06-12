package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateSwapProviderLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateSwapProviderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateSwapProviderLogic {
	return &UpdateSwapProviderLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateSwapProviderLogic) UpdateSwapProvider(in *pb.AdminUpdateSwapProviderRequest) (*pb.AdminUpdateSwapProviderResponse, error) {
	if l.svcCtx.SwapRpc == nil {
		l.Logger.Errorf("Swap service client is not available")
		return &pb.AdminUpdateSwapProviderResponse{
			Success: false,
			Message: "Swap服务未配置或未启动",
		}, nil
	}

	// 调用 Swap 服务
	swapReq := &pb.SwapUpdateProviderRequest{
		Id:               in.Id,
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

	resp, err := l.svcCtx.SwapRpc.UpdateSwapProvider(l.ctx, swapReq)
	if err != nil {
		l.Logger.Errorf("Failed to call swap.UpdateSwapProvider: %v", err)
		return &pb.AdminUpdateSwapProviderResponse{
			Success: false,
			Message: "调用Swap服务失败",
		}, nil
	}

	return &pb.AdminUpdateSwapProviderResponse{
		Success: resp.Success,
		Message: resp.Message,
	}, nil
}
