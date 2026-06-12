package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListSwapProvidersLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListSwapProvidersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListSwapProvidersLogic {
	return &ListSwapProvidersLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListSwapProvidersLogic) ListSwapProviders(in *pb.AdminListSwapProvidersRequest) (*pb.AdminListSwapProvidersResponse, error) {
	if l.svcCtx.SwapRpc == nil {
		l.Logger.Errorf("Swap service client is not available")
		return &pb.AdminListSwapProvidersResponse{
			Success: false,
			Message: "Swap服务未配置或未启动",
		}, nil
	}

	// 调用 Swap 服务
	swapReq := &pb.SwapListProvidersRequest{
		ProviderCode: in.ProviderCode,
		IsEnabled:    in.IsEnabled,
		Page:         in.Page,
		PageSize:     in.PageSize,
	}

	resp, err := l.svcCtx.SwapRpc.ListSwapProviders(l.ctx, swapReq)
	if err != nil {
		l.Logger.Errorf("Failed to call swap.ListSwapProviders: %v", err)
		return &pb.AdminListSwapProvidersResponse{
			Success: false,
			Message: "调用Swap服务失败",
		}, nil
	}

	// 转换为 Admin 格式
	items := make([]*pb.AdminSwapProviderItem, 0, len(resp.Providers))
	for _, provider := range resp.Providers {
		items = append(items, &pb.AdminSwapProviderItem{
			Id:               provider.Id,
			ProviderCode:     provider.ProviderCode,
			ProviderName:     provider.ProviderName,
			Description:      provider.Description,
			LogoUrl:          provider.LogoUrl,
			IsEnabled:        provider.IsEnabled,
			MinSwapAmountUsd: provider.MinSwapAmountUsd,
			MaxSwapAmountUsd: provider.MaxSwapAmountUsd,
			DefaultSlippage:  provider.DefaultSlippage,
			MaxSlippage:      provider.MaxSlippage,
			FeeRate:          provider.FeeRate,
			ConfigJson:       provider.ConfigJson,
			CreatedAt:        provider.CreatedAt,
			UpdatedAt:        provider.UpdatedAt,
		})
	}

	var pagination *pb.Pagination
	if resp.Pagination != nil {
		pagination = &pb.Pagination{
			Page:     resp.Pagination.Page,
			PageSize: resp.Pagination.PageSize,
			Total:    resp.Pagination.Total,
		}
	}

	return &pb.AdminListSwapProvidersResponse{
		Success:    resp.Success,
		Message:    resp.Message,
		Providers:  items,
		Pagination: pagination,
	}, nil
}
