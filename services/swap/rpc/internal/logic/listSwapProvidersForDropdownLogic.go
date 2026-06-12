package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/swap/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListSwapProvidersForDropdownLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListSwapProvidersForDropdownLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListSwapProvidersForDropdownLogic {
	return &ListSwapProvidersForDropdownLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListSwapProvidersForDropdown 获取服务商列表（用于下拉框）
func (l *ListSwapProvidersForDropdownLogic) ListSwapProvidersForDropdown(in *pb.SwapListProvidersForDropdownRequest) (*pb.SwapListProvidersForDropdownResponse, error) {
	// 查询所有启用的服务商
	providers, err := l.svcCtx.SwapProviderRepo.ListEnabled(l.ctx)
	if err != nil {
		l.Logger.Errorf("Failed to list providers for dropdown: %v", err)
		return &pb.SwapListProvidersForDropdownResponse{
			Success: false,
			Message: "查询服务商列表失败",
		}, nil
	}

	// 转换为下拉框格式
	items := make([]*pb.SwapProviderDropdownItem, 0, len(providers))
	for _, provider := range providers {
		items = append(items, &pb.SwapProviderDropdownItem{
			Id:           provider.ID,
			ProviderName: provider.ProviderName,
		})
	}

	return &pb.SwapListProvidersForDropdownResponse{
		Success:   true,
		Message:   "查询成功",
		Providers: items,
	}, nil
}
