package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetSwapProviderDetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetSwapProviderDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSwapProviderDetailLogic {
	return &GetSwapProviderDetailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetSwapProviderDetailLogic) GetSwapProviderDetail(in *pb.AdminGetSwapProviderDetailRequest) (*pb.AdminGetSwapProviderDetailResponse, error) {
	if l.svcCtx.SwapRpc == nil {
		l.Logger.Errorf("Swap service client is not available")
		return &pb.AdminGetSwapProviderDetailResponse{
			Success: false,
			Message: "Swap服务未配置或未启动",
		}, nil
	}

	// 调用 Swap 服务
	swapReq := &pb.SwapGetProviderDetailRequest{
		Id: in.Id,
	}

	resp, err := l.svcCtx.SwapRpc.GetSwapProviderDetail(l.ctx, swapReq)
	if err != nil {
		l.Logger.Errorf("Failed to call swap.GetSwapProviderDetail: %v", err)
		return &pb.AdminGetSwapProviderDetailResponse{
			Success: false,
			Message: "调用Swap服务失败",
		}, nil
	}

	var provider *pb.AdminSwapProviderItem
	if resp.Provider != nil {
		provider = &pb.AdminSwapProviderItem{
			Id:               resp.Provider.Id,
			ProviderCode:     resp.Provider.ProviderCode,
			ProviderName:     resp.Provider.ProviderName,
			Description:      resp.Provider.Description,
			LogoUrl:          resp.Provider.LogoUrl,
			IsEnabled:        resp.Provider.IsEnabled,
			MinSwapAmountUsd: resp.Provider.MinSwapAmountUsd,
			MaxSwapAmountUsd: resp.Provider.MaxSwapAmountUsd,
			DefaultSlippage:  resp.Provider.DefaultSlippage,
			MaxSlippage:      resp.Provider.MaxSlippage,
			FeeRate:          resp.Provider.FeeRate,
			ConfigJson:       resp.Provider.ConfigJson,
			CreatedAt:        resp.Provider.CreatedAt,
			UpdatedAt:        resp.Provider.UpdatedAt,
		}
	}

	return &pb.AdminGetSwapProviderDetailResponse{
		Success:  resp.Success,
		Message:  resp.Message,
		Provider: provider,
	}, nil
}
