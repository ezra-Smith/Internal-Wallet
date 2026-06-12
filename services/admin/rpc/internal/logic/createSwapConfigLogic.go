package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/svc"

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

func (l *CreateSwapConfigLogic) CreateSwapConfig(in *pb.AdminCreateSwapConfigRequest) (*pb.AdminCreateSwapConfigResponse, error) {
	if l.svcCtx.SwapRpc == nil {
		l.Logger.Errorf("Swap service client is not available")
		return &pb.AdminCreateSwapConfigResponse{
			Success: false,
			Message: "Swap服务未配置或未启动",
		}, nil
	}

	resp, err := l.svcCtx.SwapRpc.CreateSwapConfig(l.ctx, &pb.SwapCreateSwapConfigRequest{
		ProviderId:       in.ProviderId,
		ChainId:          in.ChainId,
		ChainName:        in.ChainName,
		ChainSymbol:      in.ChainSymbol,
		TokenSymbol:      in.TokenSymbol,
		TokenName:        in.TokenName,
		ContractAddress:  in.ContractAddress,
		Decimals:         in.Decimals,
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
		l.Logger.Errorf("Failed to call swap.CreateSwapConfig: %v", err)
		return &pb.AdminCreateSwapConfigResponse{
			Success: false,
			Message: "调用Swap服务失败",
		}, nil
	}

	// TODO: 记录审计日志（可选）
	// l.auditLog("create_swap_config", ...)

	return &pb.AdminCreateSwapConfigResponse{
		Success: resp.Success,
		Message: resp.Message,
		Id:      resp.Id,
	}, nil
}
