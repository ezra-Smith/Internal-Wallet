package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/svc"

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

func (l *ListProjectSwapConfigsLogic) ListProjectSwapConfigs(in *pb.AdminListProjectSwapConfigsRequest) (*pb.AdminListProjectSwapConfigsResponse, error) {
	if l.svcCtx.SwapRpc == nil {
		l.Logger.Errorf("Swap service client is not available")
		return &pb.AdminListProjectSwapConfigsResponse{
			Success: false,
			Message: "Swap服务未配置或未启动",
		}, nil
	}

	resp, err := l.svcCtx.SwapRpc.ListProjectSwapConfigs(l.ctx, &pb.SwapListProjectSwapConfigsRequest{
		ProjectName:    in.ProjectName,
		Provider:       in.Provider,
		ChainId:        in.ChainId,
		ProjectEnabled: in.ProjectEnabled,
		Page:           in.Page,
		PageSize:       in.PageSize,
	})
	if err != nil {
		l.Logger.Errorf("Failed to call swap.ListProjectSwapConfigs: %v", err)
		return &pb.AdminListProjectSwapConfigsResponse{
			Success: false,
			Message: "调用Swap服务失败",
		}, nil
	}

	return &pb.AdminListProjectSwapConfigsResponse{
		Success:    resp.Success,
		Message:    resp.Message,
		Configs:    mapProjectSwapConfigItems(resp.Configs),
		Pagination: mapSwapPagination(resp.Pagination),
	}, nil
}
