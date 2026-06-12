package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/swap/rpc/internal/model"
	"internalwallet/services/swap/rpc/internal/svc"

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

// EnableSwapConfigForProject 为项目启用Swap配置
func (l *EnableSwapConfigForProjectLogic) EnableSwapConfigForProject(in *pb.SwapEnableSwapConfigForProjectRequest) (*pb.SwapEnableSwapConfigForProjectResponse, error) {
	if in.ProjectName == "" || in.ConfigId <= 0 {
		return &pb.SwapEnableSwapConfigForProjectResponse{
			Success: false,
			Message: "项目名称或配置ID无效",
		}, nil
	}

	// 检查配置是否存在
	config, err := l.svcCtx.SwapConfigRepo.GetByID(l.ctx, in.ConfigId)
	if err != nil {
		return &pb.SwapEnableSwapConfigForProjectResponse{
			Success: false,
			Message: "配置不存在",
		}, nil
	}

	// 检查是否已启用
	existing, _ := l.svcCtx.SwapProjectEnabledRepo.GetByProjectAndConfig(
		l.ctx, in.ProjectName, in.ConfigId,
	)
	if existing != nil {
		// 已存在，更新is_enabled状态
		updates := map[string]interface{}{
			"is_enabled": in.IsEnabled,
		}
		if err := l.svcCtx.SwapProjectEnabledRepo.UpdateFields(l.ctx, existing.ID, updates); err != nil {
			l.Logger.Errorf("Failed to update project enabled config: %v", err)
			return &pb.SwapEnableSwapConfigForProjectResponse{
				Success: false,
				Message: "更新启用状态失败",
			}, nil
		}
		return &pb.SwapEnableSwapConfigForProjectResponse{
			Success: true,
			Message: "更新成功",
			Id:      existing.ID,
		}, nil
	}

	// 创建新记录
	enabled := &model.SwapProjectEnabledModel{
		ProjectName: in.ProjectName,
		ConfigID:    config.ID,
		IsEnabled:   in.IsEnabled,
	}

	if err := l.svcCtx.SwapProjectEnabledRepo.Create(l.ctx, enabled); err != nil {
		l.Logger.Errorf("Failed to enable config for project: %v", err)
		return &pb.SwapEnableSwapConfigForProjectResponse{
			Success: false,
			Message: "启用配置失败",
		}, nil
	}

	return &pb.SwapEnableSwapConfigForProjectResponse{
		Success: true,
		Message: "启用成功",
		Id:      enabled.ID,
	}, nil
}
