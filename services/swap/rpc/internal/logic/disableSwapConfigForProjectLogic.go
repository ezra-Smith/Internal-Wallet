package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/swap/rpc/internal/svc"

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

// DisableSwapConfigForProject 为项目禁用Swap配置（软删除）
func (l *DisableSwapConfigForProjectLogic) DisableSwapConfigForProject(in *pb.SwapDisableSwapConfigForProjectRequest) (*pb.SwapDisableSwapConfigForProjectResponse, error) {
	if in.Id <= 0 {
		return &pb.SwapDisableSwapConfigForProjectResponse{
			Success: false,
			Message: "启用配置ID无效",
		}, nil
	}

	// 检查是否存在
	_, err := l.svcCtx.SwapProjectEnabledRepo.GetByID(l.ctx, in.Id)
	if err != nil {
		return &pb.SwapDisableSwapConfigForProjectResponse{
			Success: false,
			Message: "配置不存在",
		}, nil
	}

	// 软删除
	if err := l.svcCtx.SwapProjectEnabledRepo.SoftDelete(l.ctx, in.Id); err != nil {
		l.Logger.Errorf("Failed to disable project config %d: %v", in.Id, err)
		return &pb.SwapDisableSwapConfigForProjectResponse{
			Success: false,
			Message: "禁用配置失败",
		}, nil
	}

	return &pb.SwapDisableSwapConfigForProjectResponse{
		Success: true,
		Message: "禁用成功",
	}, nil
}
