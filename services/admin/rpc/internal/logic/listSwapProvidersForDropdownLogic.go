package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/svc"

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

// ListSwapProvidersForDropdown 获取服务商列表（用于下拉框，门面模式）
func (l *ListSwapProvidersForDropdownLogic) ListSwapProvidersForDropdown(in *pb.AdminListSwapProvidersForDropdownRequest) (*pb.AdminListSwapProvidersForDropdownResponse, error) {
	if l.svcCtx.SwapRpc == nil {
		l.Logger.Errorf("Swap service client is not available")
		return &pb.AdminListSwapProvidersForDropdownResponse{
			Success: false,
			Message: "Swap服务未配置或未启动",
		}, nil
	}

	// 调用 Swap 服务
	resp, err := l.svcCtx.SwapRpc.ListSwapProvidersForDropdown(l.ctx, &pb.SwapListProvidersForDropdownRequest{})
	if err != nil {
		l.Logger.Errorf("Failed to call swap.ListSwapProvidersForDropdown: %v", err)
		return &pb.AdminListSwapProvidersForDropdownResponse{
			Success: false,
			Message: "调用Swap服务失败",
		}, nil
	}

	// 转换为 Admin 格式（虽然结构相同，但为了类型安全）
	items := make([]*pb.AdminSwapProviderDropdownItem, 0, len(resp.Providers))
	for _, provider := range resp.Providers {
		items = append(items, &pb.AdminSwapProviderDropdownItem{
			Id:           provider.Id,
			ProviderName: provider.ProviderName,
		})
	}

	return &pb.AdminListSwapProvidersForDropdownResponse{
		Success:   resp.Success,
		Message:   resp.Message,
		Providers: items,
	}, nil
}
