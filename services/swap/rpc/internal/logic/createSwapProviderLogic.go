package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/swap/rpc/internal/model"
	"internalwallet/services/swap/rpc/internal/svc"

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

// CreateSwapProvider 创建Swap服务商
func (l *CreateSwapProviderLogic) CreateSwapProvider(in *pb.SwapCreateProviderRequest) (*pb.SwapCreateProviderResponse, error) {
	// 参数验证
	if in.ProviderCode == "" || in.ProviderName == "" {
		return &pb.SwapCreateProviderResponse{
			Success: false,
			Message: "必填参数缺失：provider_code, provider_name",
		}, nil
	}

	// 检查是否已存在
	existing, _ := l.svcCtx.SwapProviderRepo.GetByCode(l.ctx, in.ProviderCode)
	if existing != nil {
		return &pb.SwapCreateProviderResponse{
			Success: false,
			Message: "该服务商代码已存在",
		}, nil
	}

	// 创建Model
	provider := &model.SwapProviderModel{
		ProviderCode:     in.ProviderCode,
		ProviderName:     in.ProviderName,
		Description:      in.Description,
		LogoURL:          in.LogoUrl,
		IsEnabled:        in.IsEnabled,
		MinSwapAmountUsd: in.MinSwapAmountUsd,
		MaxSwapAmountUsd: in.MaxSwapAmountUsd,
		DefaultSlippage:  in.DefaultSlippage,
		MaxSlippage:      in.MaxSlippage,
		FeeRate:          in.FeeRate,
		ConfigJSON:       in.ConfigJson,
	}

	// 设置默认值
	if provider.MinSwapAmountUsd == "" {
		provider.MinSwapAmountUsd = "10"
	}
	if provider.MaxSwapAmountUsd == "" {
		provider.MaxSwapAmountUsd = "100000"
	}
	if provider.DefaultSlippage == "" {
		provider.DefaultSlippage = "0.5"
	}
	if provider.MaxSlippage == "" {
		provider.MaxSlippage = "5.0"
	}
	if provider.FeeRate == "" {
		provider.FeeRate = "0.25"
	}
	if provider.ConfigJSON == "" {
		provider.ConfigJSON = "[]"
	}

	// 保存
	if err := l.svcCtx.SwapProviderRepo.Create(l.ctx, provider); err != nil {
		l.Logger.Errorf("Failed to create swap provider: %v", err)
		return &pb.SwapCreateProviderResponse{
			Success: false,
			Message: "创建服务商失败",
		}, nil
	}

	return &pb.SwapCreateProviderResponse{
		Success: true,
		Message: "创建成功",
		Id:      provider.ID,
	}, nil
}
