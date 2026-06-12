package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/swap/rpc/internal/model"
	"internalwallet/services/swap/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateSwapConfigLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateSwapConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateSwapConfigLogic {
	return &CreateSwapConfigLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// inheritParam 参数继承：如果值为空则使用默认值
func inheritParam(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}

// CreateSwapConfig 创建全局Swap配置
func (l *CreateSwapConfigLogic) CreateSwapConfig(in *pb.SwapCreateSwapConfigRequest) (*pb.SwapCreateSwapConfigResponse, error) {
	// 参数验证
	if in.ProviderId <= 0 || in.ChainId <= 0 || in.TokenSymbol == "" {
		return &pb.SwapCreateSwapConfigResponse{
			Success: false,
			Message: "必填参数缺失：provider_id, chain_id, token_symbol",
		}, nil
	}

	// 验证provider_id存在
	provider, err := l.svcCtx.SwapProviderRepo.GetByID(l.ctx, in.ProviderId)
	if err != nil || provider == nil {
		return &pb.SwapCreateSwapConfigResponse{
			Success: false,
			Message: "服务商不存在",
		}, nil
	}

	// 检查是否已存在（同provider+chain+contract唯一）
	// 使用provider_code查询
	existing, _ := l.svcCtx.SwapConfigRepo.GetByProviderChainToken(
		l.ctx, provider.ProviderCode, in.ChainId, in.ContractAddress,
	)
	if existing != nil {
		return &pb.SwapCreateSwapConfigResponse{
			Success: false,
			Message: "该配置已存在",
		}, nil
	}

	//额外的Json配置，如果为空，给一个Json格式的空对象
	if in.ConfigJson == "" {
		in.ConfigJson = "[]"
	}

	// 创建Model（使用参数继承）
	config := &model.SwapConfigModel{
		ProviderID:       in.ProviderId,
		Provider:         provider.ProviderCode, // 冗余字段
		ProviderName:     provider.ProviderName, // 冗余字段
		ChainID:          in.ChainId,
		ChainName:        in.ChainName,
		ChainSymbol:      in.ChainSymbol,
		TokenSymbol:      in.TokenSymbol,
		TokenName:        in.TokenName,
		ContractAddress:  in.ContractAddress,
		Decimals:         int(in.Decimals),
		IconURL:          in.IconUrl,
		IsEnabled:        in.IsEnabled,
		Priority:         int(in.Priority),
		RouterAddress:    in.RouterAddress,
		MinSwapAmountUsd: inheritParam(in.MinSwapAmountUsd, provider.MinSwapAmountUsd),
		MaxSwapAmountUsd: inheritParam(in.MaxSwapAmountUsd, provider.MaxSwapAmountUsd),
		DefaultSlippage:  inheritParam(in.DefaultSlippage, provider.DefaultSlippage),
		MaxSlippage:      inheritParam(in.MaxSlippage, provider.MaxSlippage),
		FeeRate:          inheritParam(in.FeeRate, provider.FeeRate),
		ConfigJSON:       in.ConfigJson,
	}

	// 保存
	if err := l.svcCtx.SwapConfigRepo.Create(l.ctx, config); err != nil {
		l.Logger.Errorf("Failed to create swap config: %v", err)
		return &pb.SwapCreateSwapConfigResponse{
			Success: false,
			Message: "创建配置失败",
		}, nil
	}

	return &pb.SwapCreateSwapConfigResponse{
		Success: true,
		Message: "创建成功",
		Id:      config.ID,
	}, nil
}
