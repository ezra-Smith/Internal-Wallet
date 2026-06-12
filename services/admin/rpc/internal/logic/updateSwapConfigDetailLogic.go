package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateSwapConfigDetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateSwapConfigDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateSwapConfigDetailLogic {
	return &UpdateSwapConfigDetailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateSwapConfigDetailLogic) UpdateSwapConfigDetail(in *pb.AdminUpdateSwapConfigDetailRequest) (*pb.AdminUpdateSwapConfigDetailResponse, error) {
	if l.svcCtx.SwapRpc == nil {
		l.Logger.Errorf("Swap service client is not available")
		return &pb.AdminUpdateSwapConfigDetailResponse{
			Success: false,
			Message: "Swap服务未配置或未启动",
		}, nil
	}

	resp, err := l.svcCtx.SwapRpc.UpdateSwapConfigDetail(l.ctx, &pb.SwapUpdateSwapConfigDetailRequest{
		Id:               in.Id,
		ProviderName:     in.ProviderName,
		ChainName:        in.ChainName,
		ChainSymbol:      in.ChainSymbol,
		TokenName:        in.TokenName,
		IconUrl:          in.IconUrl,
		IsEnabled:        in.IsEnabled,
		Priority:         in.Priority,
		RouterAddress:    in.RouterAddress,
		MinSwapAmountUsd: in.MinSwapAmountUsd,
		MaxSwapAmountUsd: in.MaxSwapAmountUsd,
		DefaultSlippage:  in.DefaultSlippage,
		MaxSlippage:      in.MaxSlippage,
		FeeRate:          in.FeeRate,
		ConfigJson:       in.ConfigJson,
	})
	if err != nil {
		l.Logger.Errorf("Failed to call swap.UpdateSwapConfigDetail: %v", err)
		return &pb.AdminUpdateSwapConfigDetailResponse{
			Success: false,
			Message: "调用Swap服务失败",
		}, nil
	}

	return &pb.AdminUpdateSwapConfigDetailResponse{
		Success: resp.Success,
		Message: resp.Message,
	}, nil
}
