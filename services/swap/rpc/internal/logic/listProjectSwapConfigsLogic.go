package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/swap/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListProjectSwapConfigsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListProjectSwapConfigsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListProjectSwapConfigsLogic {
	return &ListProjectSwapConfigsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListProjectSwapConfigs 列出项目的Swap配置（联表查询）
func (l *ListProjectSwapConfigsLogic) ListProjectSwapConfigs(in *pb.SwapListProjectSwapConfigsRequest) (*pb.SwapListProjectSwapConfigsResponse, error) {
	if in.ProjectName == "" {
		return &pb.SwapListProjectSwapConfigsResponse{
			Success: false,
			Message: "项目名称不能为空",
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

	// 查询项目配置（联表）
	results, total, err := l.svcCtx.SwapProjectEnabledRepo.ListProjectConfigsWithDetails(
		l.ctx, in.ProjectName, int32(page), int32(pageSize),
	)
	if err != nil {
		l.Logger.Errorf("Failed to list project swap configs for %s: %v", in.ProjectName, err)
		return &pb.SwapListProjectSwapConfigsResponse{
			Success: false,
			Message: "查询项目配置失败",
		}, nil
	}

	// 转换为pb（从map[string]interface{}解析）
	items := make([]*pb.ProjectSwapConfigItem, 0, len(results))
	for _, row := range results {
		item := &pb.ProjectSwapConfigItem{
			EnabledId:        row["enabled_id"].(int64),
			ProjectName:      row["project_name"].(string),
			ConfigId:         row["config_id"].(int64),
			ProjectEnabled:   parseBool(row["project_enabled"]),
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

	return &pb.SwapListProjectSwapConfigsResponse{
		Success:    true,
		Message:    "查询成功",
		Configs:    items,
		Pagination: pagination,
	}, nil
}
