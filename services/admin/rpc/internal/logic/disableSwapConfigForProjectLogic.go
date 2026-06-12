package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type DisableSwapConfigForProjectLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDisableSwapConfigForProjectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DisableSwapConfigForProjectLogic {
	return &DisableSwapConfigForProjectLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DisableSwapConfigForProjectLogic) DisableSwapConfigForProject(in *pb.AdminDisableSwapConfigForProjectRequest) (*pb.AdminDisableSwapConfigForProjectResponse, error) {
	if l.svcCtx.SwapRpc == nil {
		l.Logger.Errorf("Swap service client is not available")
		return &pb.AdminDisableSwapConfigForProjectResponse{
			Success: false,
			Message: "Swap服务未配置或未启动",
		}, nil
	}

	resp, err := l.svcCtx.SwapRpc.DisableSwapConfigForProject(l.ctx, &pb.SwapDisableSwapConfigForProjectRequest{
		Id: in.Id,
	})
	if err != nil {
		l.Logger.Errorf("Failed to call swap.DisableSwapConfigForProject: %v", err)
		return &pb.AdminDisableSwapConfigForProjectResponse{
			Success: false,
			Message: "调用Swap服务失败",
		}, nil
	}

	return &pb.AdminDisableSwapConfigForProjectResponse{
		Success: resp.Success,
		Message: resp.Message,
	}, nil
}
