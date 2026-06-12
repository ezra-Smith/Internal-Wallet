package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/swap/rpc/internal/svc"

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

// GetSwapProviderDetail 获取Swap服务商详情
func (l *GetSwapProviderDetailLogic) GetSwapProviderDetail(in *pb.SwapGetProviderDetailRequest) (*pb.SwapGetProviderDetailResponse, error) {
	// 参数验证
	if in.Id <= 0 {
		return &pb.SwapGetProviderDetailResponse{
			Success: false,
			Message: "无效的ID",
		}, nil
	}

	// 查询
	provider, err := l.svcCtx.SwapProviderRepo.GetByID(l.ctx, in.Id)
	if err != nil {
		l.Logger.Errorf("Failed to get swap provider: %v", err)
		return &pb.SwapGetProviderDetailResponse{
			Success: false,
			Message: "查询服务商失败",
		}, nil
	}
	if provider == nil {
		return &pb.SwapGetProviderDetailResponse{
			Success: false,
			Message: "服务商不存在",
		}, nil
	}

	// 转换为pb
	item := &pb.SwapProviderItem{
		Id:               provider.ID,
		ProviderCode:     provider.ProviderCode,
		ProviderName:     provider.ProviderName,
		Description:      provider.Description,
		LogoUrl:          provider.LogoURL,
		IsEnabled:        provider.IsEnabled,
		MinSwapAmountUsd: provider.MinSwapAmountUsd,
		MaxSwapAmountUsd: provider.MaxSwapAmountUsd,
		DefaultSlippage:  provider.DefaultSlippage,
		MaxSlippage:      provider.MaxSlippage,
		FeeRate:          provider.FeeRate,
		ConfigJson:       provider.ConfigJSON,
		CreatedAt:        provider.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:        provider.UpdatedAt.Format("2006-01-02 15:04:05"),
	}

	return &pb.SwapGetProviderDetailResponse{
		Success:  true,
		Message:  "ok",
		Provider: item,
	}, nil
}
