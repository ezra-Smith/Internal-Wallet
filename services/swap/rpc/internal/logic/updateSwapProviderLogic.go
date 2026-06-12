package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/swap/rpc/internal/svc"

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

// UpdateSwapProvider 更新Swap服务商
func (l *UpdateSwapProviderLogic) UpdateSwapProvider(in *pb.SwapUpdateProviderRequest) (*pb.SwapUpdateProviderResponse, error) {
	// 参数验证
	if in.Id <= 0 {
		return &pb.SwapUpdateProviderResponse{
			Success: false,
			Message: "无效的ID",
		}, nil
	}

	// 检查是否存在
	existing, err := l.svcCtx.SwapProviderRepo.GetByID(l.ctx, in.Id)
	if err != nil {
		l.Logger.Errorf("Failed to get provider: %v", err)
		return &pb.SwapUpdateProviderResponse{
			Success: false,
			Message: "服务商不存在",
		}, nil
	}
	if existing == nil {
		return &pb.SwapUpdateProviderResponse{
			Success: false,
			Message: "服务商不存在",
		}, nil
	}

	// 构建更新字段
	fields := make(map[string]interface{})
	if in.ProviderName != "" {
		fields["provider_name"] = in.ProviderName
	}
	if in.Description != "" {
		fields["description"] = in.Description
	}
	if in.LogoUrl != "" {
		fields["logo_url"] = in.LogoUrl
	}
	fields["is_enabled"] = in.IsEnabled
	if in.MinSwapAmountUsd != "" {
		fields["min_swap_amount_usd"] = in.MinSwapAmountUsd
	}
	if in.MaxSwapAmountUsd != "" {
		fields["max_swap_amount_usd"] = in.MaxSwapAmountUsd
	}
	if in.DefaultSlippage != "" {
		fields["default_slippage"] = in.DefaultSlippage
	}
	if in.MaxSlippage != "" {
		fields["max_slippage"] = in.MaxSlippage
	}
	if in.FeeRate != "" {
		fields["fee_rate"] = in.FeeRate
	}
	if in.ConfigJson != "" {
		fields["config_json"] = in.ConfigJson
	}

	// 更新
	if err := l.svcCtx.SwapProviderRepo.UpdateFields(l.ctx, in.Id, fields); err != nil {
		l.Logger.Errorf("Failed to update swap provider: %v", err)
		return &pb.SwapUpdateProviderResponse{
			Success: false,
			Message: "更新服务商失败",
		}, nil
	}

	return &pb.SwapUpdateProviderResponse{
		Success: true,
		Message: "更新成功",
	}, nil
}
