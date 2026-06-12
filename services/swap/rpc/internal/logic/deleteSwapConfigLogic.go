package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/swap/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteSwapConfigLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteSwapConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteSwapConfigLogic {
	return &DeleteSwapConfigLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// DeleteSwapConfig 删除Swap配置（软删除）
func (l *DeleteSwapConfigLogic) DeleteSwapConfig(in *pb.SwapDeleteSwapConfigRequest) (*pb.SwapDeleteSwapConfigResponse, error) {
	if in.Id <= 0 {
		return &pb.SwapDeleteSwapConfigResponse{
			Success: false,
			Message: "配置ID无效",
		}, nil
	}

	// 检查配置是否存在
	_, err := l.svcCtx.SwapConfigRepo.GetByID(l.ctx, in.Id)
	if err != nil {
		return &pb.SwapDeleteSwapConfigResponse{
			Success: false,
			Message: "配置不存在",
		}, nil
	}

	// 软删除
	if err := l.svcCtx.SwapConfigRepo.SoftDelete(l.ctx, in.Id); err != nil {
		l.Logger.Errorf("Failed to delete swap config %d: %v", in.Id, err)
		return &pb.SwapDeleteSwapConfigResponse{
			Success: false,
			Message: "删除配置失败",
		}, nil
	}

	return &pb.SwapDeleteSwapConfigResponse{
		Success: true,
		Message: "删除成功",
	}, nil
}
