package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/swap/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListSwapConfigsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListSwapConfigsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListSwapConfigsLogic {
	return &ListSwapConfigsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListSwapConfigs 列出全局Swap配置（扁平化）
func (l *ListSwapConfigsLogic) ListSwapConfigs(in *pb.SwapListSwapConfigsRequest) (*pb.SwapListSwapConfigsResponse, error) {
	// 构建过滤条件
	filters := make(map[string]interface{})
	if in.ProviderId != 0 {
		filters["provider_id"] = in.ProviderId
	}
	if in.ChainId != 0 {
		filters["chain_id"] = in.ChainId
	}
	if in.TokenSymbol != "" {
		filters["token_symbol"] = in.TokenSymbol
	}
	// is_enabled过滤（proto bool默认false，这里根据实际需求处理）
	// 如果需要过滤enabled状态，可以添加：filters["is_enabled"] = in.IsEnabled

	// 分页参数
	page := int(in.Page)
	if page < 1 {
		page = 1
	}
	pageSize := int(in.PageSize)
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	// 查询
	configs, total, err := l.svcCtx.SwapConfigRepo.List(l.ctx, filters, int32(page), int32(pageSize))
	if err != nil {
		l.Logger.Errorf("Failed to list swap configs: %v", err)
		return &pb.SwapListSwapConfigsResponse{
			Success: false,
			Message: "查询配置失败",
		}, nil
	}

	// 转换为pb
	items := make([]*pb.SwapConfigItem, 0, len(configs))
	for _, cfg := range configs {
		items = append(items, &pb.SwapConfigItem{
			Id:               cfg.ID,
			ProviderId:       cfg.ProviderID,
			Provider:         cfg.Provider,
			ProviderName:     cfg.ProviderName,
			ChainId:          cfg.ChainID,
			ChainName:        cfg.ChainName,
			ChainSymbol:      cfg.ChainSymbol,
			TokenSymbol:      cfg.TokenSymbol,
			TokenName:        cfg.TokenName,
			ContractAddress:  cfg.ContractAddress,
			Decimals:         int32(cfg.Decimals),
			IconUrl:          cfg.IconURL,
			IsEnabled:        cfg.IsEnabled,
			Priority:         int32(cfg.Priority),
			RouterAddress:    cfg.RouterAddress,
			MinSwapAmountUsd: cfg.MinSwapAmountUsd,
			MaxSwapAmountUsd: cfg.MaxSwapAmountUsd,
			DefaultSlippage:  cfg.DefaultSlippage,
			MaxSlippage:      cfg.MaxSlippage,
			FeeRate:          cfg.FeeRate,
			ConfigJson:       cfg.ConfigJSON,
			CreatedAt:        cfg.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt:        cfg.UpdatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	// 分页信息
	totalPages := int32((total + int64(pageSize) - 1) / int64(pageSize))
	pagination := &pb.SwapPagination{
		Page:       int32(page),
		PageSize:   int32(pageSize),
		Total:      total,
		TotalPages: totalPages,
		HasNext:    int32(page) < totalPages,
		HasPrev:    page > 1,
	}

	return &pb.SwapListSwapConfigsResponse{
		Success:    true,
		Message:    "查询成功",
		Configs:    items,
		Pagination: pagination,
	}, nil
}
