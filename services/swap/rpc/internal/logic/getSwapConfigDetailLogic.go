package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/swap/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetSwapConfigDetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetSwapConfigDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSwapConfigDetailLogic {
	return &GetSwapConfigDetailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetSwapConfigDetail 获取单个Swap配置详情
func (l *GetSwapConfigDetailLogic) GetSwapConfigDetail(in *pb.SwapGetSwapConfigDetailRequest) (*pb.SwapGetSwapConfigDetailResponse, error) {
	if in.Id <= 0 {
		return &pb.SwapGetSwapConfigDetailResponse{
			Success: false,
			Message: "配置ID无效",
		}, nil
	}

	config, err := l.svcCtx.SwapConfigRepo.GetByID(l.ctx, in.Id)
	if err != nil {
		l.Logger.Errorf("Failed to get swap config by id %d: %v", in.Id, err)
		return &pb.SwapGetSwapConfigDetailResponse{
			Success: false,
			Message: "配置不存在",
		}, nil
	}

	return &pb.SwapGetSwapConfigDetailResponse{
		Success: true,
		Message: "查询成功",
		Config: &pb.SwapConfigItem{
			Id:               config.ID,
			ProviderId:       config.ProviderID,
			Provider:         config.Provider,
			ProviderName:     config.ProviderName,
			ChainId:          config.ChainID,
			ChainName:        config.ChainName,
			ChainSymbol:      config.ChainSymbol,
			TokenSymbol:      config.TokenSymbol,
			TokenName:        config.TokenName,
			ContractAddress:  config.ContractAddress,
			Decimals:         int32(config.Decimals),
			IconUrl:          config.IconURL,
			IsEnabled:        config.IsEnabled,
			Priority:         int32(config.Priority),
			RouterAddress:    config.RouterAddress,
			MinSwapAmountUsd: config.MinSwapAmountUsd,
			MaxSwapAmountUsd: config.MaxSwapAmountUsd,
			DefaultSlippage:  config.DefaultSlippage,
			MaxSlippage:      config.MaxSlippage,
			FeeRate:          config.FeeRate,
			ConfigJson:       config.ConfigJSON,
			CreatedAt:        config.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt:        config.UpdatedAt.Format("2006-01-02 15:04:05"),
		},
	}, nil
}
