package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/swap/rpc/internal/svc"

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

// ListSwapProviders 列出Swap服务商
func (l *ListSwapProvidersLogic) ListSwapProviders(in *pb.SwapListProvidersRequest) (*pb.SwapListProvidersResponse, error) {
	// 分页参数
	page := in.Page
	if page < 1 {
		page = 1
	}
	pageSize := in.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	// 构建过滤器
	filters := make(map[string]interface{})
	if in.ProviderCode != "" {
		filters["provider_code"] = in.ProviderCode
	}
	// 只在明确指定时才过滤is_enabled
	if in.IsEnabled {
		filters["is_enabled"] = true
	}

	// 查询
	providers, total, err := l.svcCtx.SwapProviderRepo.List(l.ctx, filters, page, pageSize)
	if err != nil {
		l.Logger.Errorf("Failed to list swap providers: %v", err)
		return &pb.SwapListProvidersResponse{
			Success: false,
			Message: "查询服务商列表失败",
		}, nil
	}

	// 转换为pb
	items := make([]*pb.SwapProviderItem, 0, len(providers))
	for _, p := range providers {
		item := &pb.SwapProviderItem{
			Id:               p.ID,
			ProviderCode:     p.ProviderCode,
			ProviderName:     p.ProviderName,
			Description:      p.Description,
			LogoUrl:          p.LogoURL,
			IsEnabled:        p.IsEnabled,
			MinSwapAmountUsd: p.MinSwapAmountUsd,
			MaxSwapAmountUsd: p.MaxSwapAmountUsd,
			DefaultSlippage:  p.DefaultSlippage,
			MaxSlippage:      p.MaxSlippage,
			FeeRate:          p.FeeRate,
			ConfigJson:       p.ConfigJSON,
			CreatedAt:        p.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt:        p.UpdatedAt.Format("2006-01-02 15:04:05"),
		}
		items = append(items, item)
	}

	// 分页信息
	totalPages := int32((total + int64(pageSize) - 1) / int64(pageSize))
	pagination := &pb.SwapPagination{
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: totalPages,
		HasNext:    page < totalPages,
		HasPrev:    page > 1,
	}

	return &pb.SwapListProvidersResponse{
		Success:    true,
		Message:    "ok",
		Providers:  items,
		Pagination: pagination,
	}, nil
}
