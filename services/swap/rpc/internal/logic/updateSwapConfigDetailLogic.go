package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/swap/rpc/internal/svc"

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

// UpdateSwapConfigDetail 更新Swap配置
func (l *UpdateSwapConfigDetailLogic) UpdateSwapConfigDetail(in *pb.SwapUpdateSwapConfigDetailRequest) (*pb.SwapUpdateSwapConfigDetailResponse, error) {
	if in.Id <= 0 {
		return &pb.SwapUpdateSwapConfigDetailResponse{
			Success: false,
			Message: "配置ID无效",
		}, nil
	}

	// 检查配置是否存在
	existing, err := l.svcCtx.SwapConfigRepo.GetByID(l.ctx, in.Id)
	if err != nil {
		return &pb.SwapUpdateSwapConfigDetailResponse{
			Success: false,
			Message: "配置不存在",
		}, nil
	}

	// 构建更新字段
	updates := make(map[string]interface{})

	// 只更新非零值字段
	if in.ProviderName != "" {
		updates["provider_name"] = in.ProviderName
	}
	if in.ChainName != "" {
		updates["chain_name"] = in.ChainName
	}
	if in.ChainSymbol != "" {
		updates["chain_symbol"] = in.ChainSymbol
	}
	if in.TokenName != "" {
		updates["token_name"] = in.TokenName
	}
	if in.IconUrl != "" {
		updates["icon_url"] = in.IconUrl
	}
	if in.RouterAddress != "" {
		updates["router_address"] = in.RouterAddress
	}
	if in.MinSwapAmountUsd != "" {
		updates["min_swap_amount_usd"] = in.MinSwapAmountUsd
	}
	if in.MaxSwapAmountUsd != "" {
		updates["max_swap_amount_usd"] = in.MaxSwapAmountUsd
	}
	if in.DefaultSlippage != "" {
		updates["default_slippage"] = in.DefaultSlippage
	}
	if in.MaxSlippage != "" {
		updates["max_slippage"] = in.MaxSlippage
	}
	if in.FeeRate != "" {
		updates["fee_rate"] = in.FeeRate
	}
	if in.ConfigJson != "" {
		updates["config_json"] = in.ConfigJson
	}

	// 注意：bool字段需要特殊处理（proto默认false）
	// 这里简单处理：总是更新
	updates["is_enabled"] = in.IsEnabled

	if in.Priority != 0 {
		updates["priority"] = int(in.Priority)
	}

	if len(updates) == 0 {
		return &pb.SwapUpdateSwapConfigDetailResponse{
			Success: false,
			Message: "没有需要更新的字段",
		}, nil
	}

	// 执行更新
	if err := l.svcCtx.SwapConfigRepo.UpdateFields(l.ctx, existing.ID, updates); err != nil {
		l.Logger.Errorf("Failed to update swap config %d: %v", in.Id, err)
		return &pb.SwapUpdateSwapConfigDetailResponse{
			Success: false,
			Message: "更新配置失败",
		}, nil
	}

	return &pb.SwapUpdateSwapConfigDetailResponse{
		Success: true,
		Message: "更新成功",
	}, nil
}
