package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type EnableSwapConfigForProjectLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewEnableSwapConfigForProjectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EnableSwapConfigForProjectLogic {
	return &EnableSwapConfigForProjectLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *EnableSwapConfigForProjectLogic) EnableSwapConfigForProject(in *pb.AdminEnableSwapConfigForProjectRequest) (*pb.AdminEnableSwapConfigForProjectResponse, error) {
	if l.svcCtx.SwapRpc == nil {
		l.Logger.Errorf("Swap service client is not available")
		return &pb.AdminEnableSwapConfigForProjectResponse{
			Success: false,
			Message: "Swap服务未配置或未启动",
		}, nil
	}

	resp, err := l.svcCtx.SwapRpc.EnableSwapConfigForProject(l.ctx, &pb.SwapEnableSwapConfigForProjectRequest{
		ProjectName: in.ProjectName,
		ConfigId:    in.ConfigId,
		IsEnabled:   in.IsEnabled,
	})
	if err != nil {
		l.Logger.Errorf("Failed to call swap.EnableSwapConfigForProject: %v", err)
		return &pb.AdminEnableSwapConfigForProjectResponse{
			Success: false,
			Message: "调用Swap服务失败",
		}, nil
	}

	return &pb.AdminEnableSwapConfigForProjectResponse{
		Success: resp.Success,
		Message: resp.Message,
		Id:      resp.Id,
	}, nil
}
