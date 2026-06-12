package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/swap/rpc/internal/interceptor"
	"internalwallet/services/swap/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetMySwapConfigsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetMySwapConfigsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMySwapConfigsLogic {
	return &GetMySwapConfigsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetMySwapConfigs 业务端查询自己项目的生效配置
// 从 API Key context 自动获取 project_name，无需业务端传递
func (l *GetMySwapConfigsLogic) GetMySwapConfigs(in *pb.SwapGetMySwapConfigsRequest) (*pb.SwapGetMySwapConfigsResponse, error) {
	// 从 context 中获取 project_name（由 API Key 认证拦截器注入）
	projectName := interceptor.GetProjectName(l.ctx)
	if projectName == "" {
		return &pb.SwapGetMySwapConfigsResponse{
			Success: false,
			Message: "未找到项目信息，请使用有效的 API Key",
		}, nil
	}

	// 分页参数
	page := int(in.Page)
	if page < 1 {
		page = 1
	}
	pageSize := int(in.PageSize)
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	// 查询项目配置（联表查询，只返回已启用的配置）
	// 这里复用 SwapProjectEnabledRepo 的查询方法
	results, _, err := l.svcCtx.SwapProjectEnabledRepo.ListProjectConfigsWithDetails(
		l.ctx, projectName, int32(page), int32(pageSize),
	)
	if err != nil {
		l.Logger.Errorf("Failed to get swap configs for project %s: %v", projectName, err)
		return &pb.SwapGetMySwapConfigsResponse{
			Success: false,
			Message: "查询配置失败",
		}, nil
	}

	// 可选过滤器：provider、chain_id
	filteredResults := make([]map[string]interface{}, 0, len(results))
	for _, row := range results {
		// 只返回启用的配置
		if !parseBool(row["project_enabled"]) {
			continue
		}

		// 过滤 provider
		if in.Provider != "" && row["provider"].(string) != in.Provider {
			continue
		}

		// 过滤 chain_id
		if in.ChainId > 0 && row["chain_id"].(int64) != in.ChainId {
			continue
		}

		filteredResults = append(filteredResults, row)
	}

	// 转换为pb
	items := make([]*pb.ProjectSwapConfigItem, 0, len(filteredResults))
	for _, row := range filteredResults {
		item := &pb.ProjectSwapConfigItem{
			EnabledId:        row["enabled_id"].(int64),
			ProjectName:      row["project_name"].(string),
			ConfigId:         row["config_id"].(int64),
			ProjectEnabled:   parseBool(row["project_enabled"]),
			ProviderId:       parseInt64(row["provider_id"]),
			Provider:         row["provider"].(string),
			ProviderName:     row["provider_name"].(string),
			ChainId:          row["chain_id"].(int64),
			ChainName:        row["chain_name"].(string),
			ChainSymbol:      row["chain_symbol"].(string),
			TokenSymbol:      row["token_symbol"].(string),
			TokenName:        row["token_name"].(string),
			ContractAddress:  row["contract_address"].(string),
			Decimals:         int32(parseInt64(row["decimals"])),
			IconUrl:          row["icon_url"].(string),
			GlobalEnabled:    parseBool(row["global_enabled"]),
			Priority:         int32(parseInt64(row["priority"])),
			RouterAddress:    row["router_address"].(string),
			MinSwapAmountUsd: parseString(row["min_swap_amount_usd"]),
			MaxSwapAmountUsd: parseString(row["max_swap_amount_usd"]),
			DefaultSlippage:  parseString(row["default_slippage"]),
			MaxSlippage:      parseString(row["max_slippage"]),
			FeeRate:          parseString(row["fee_rate"]),
			CreatedAt:        parseString(row["created_at"]),
			UpdatedAt:        parseString(row["updated_at"]),
		}
		items = append(items, item)
	}

	// 分页信息（基于过滤后的结果）
	filteredTotal := int64(len(filteredResults))
	totalPages := int32((filteredTotal + int64(pageSize) - 1) / int64(pageSize))
	pagination := &pb.SwapPagination{
		Page:       int32(page),
		PageSize:   int32(pageSize),
		Total:      filteredTotal,
		TotalPages: totalPages,
		HasNext:    int32(page) < totalPages,
		HasPrev:    page > 1,
	}

	return &pb.SwapGetMySwapConfigsResponse{
		Success:    true,
		Message:    "ok",
		Configs:    items,
		Pagination: pagination,
	}, nil
}
