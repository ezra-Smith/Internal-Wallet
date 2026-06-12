package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/svc"

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

// ListSwapConfigs 列出全局Swap配置（门面，调用Swap服务）
func (l *ListSwapConfigsLogic) ListSwapConfigs(in *pb.AdminListSwapConfigsRequest) (*pb.AdminListSwapConfigsResponse, error) {
	if l.svcCtx.SwapRpc == nil {
		l.Logger.Errorf("Swap service client is not available")
		return &pb.AdminListSwapConfigsResponse{
			Success: false,
			Message: "Swap服务未配置或未启动",
		}, nil
	}

	swapReq := &pb.SwapListSwapConfigsRequest{
		ProviderId:  in.ProviderId,
		ChainId:     in.ChainId,
		TokenSymbol: in.TokenSymbol,
		IsEnabled:   in.IsEnabled,
		Page:        in.Page,
		PageSize:    in.PageSize,
	}

	resp, err := l.svcCtx.SwapRpc.ListSwapConfigs(l.ctx, swapReq)
	if err != nil {
		l.Logger.Errorf("Failed to call swap.ListSwapConfigs: %v", err)
		return &pb.AdminListSwapConfigsResponse{
			Success: false,
			Message: "调用Swap服务失败",
		}, nil
	}

	return &pb.AdminListSwapConfigsResponse{
		Success:    resp.Success,
		Message:    resp.Message,
		Configs:    mapSwapConfigItems(resp.Configs),
		Pagination: mapSwapPagination(resp.Pagination),
	}, nil
}
