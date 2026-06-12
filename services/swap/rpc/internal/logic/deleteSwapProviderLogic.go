package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/swap/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteSwapProviderLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteSwapProviderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteSwapProviderLogic {
	return &DeleteSwapProviderLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// DeleteSwapProvider 删除Swap服务商
func (l *DeleteSwapProviderLogic) DeleteSwapProvider(in *pb.SwapDeleteProviderRequest) (*pb.SwapDeleteProviderResponse, error) {
	// 参数验证
	if in.Id <= 0 {
		return &pb.SwapDeleteProviderResponse{
			Success: false,
			Message: "无效的ID",
		}, nil
	}

	// 检查是否存在
	existing, err := l.svcCtx.SwapProviderRepo.GetByID(l.ctx, in.Id)
	if err != nil {
		l.Logger.Errorf("Failed to get provider: %v", err)
		return &pb.SwapDeleteProviderResponse{
			Success: false,
			Message: "服务商不存在",
		}, nil
	}
	if existing == nil {
		return &pb.SwapDeleteProviderResponse{
			Success: false,
			Message: "服务商不存在",
		}, nil
	}

	// 软删除
	if err := l.svcCtx.SwapProviderRepo.SoftDelete(l.ctx, in.Id); err != nil {
		l.Logger.Errorf("Failed to delete swap provider: %v", err)
		return &pb.SwapDeleteProviderResponse{
			Success: false,
			Message: "删除服务商失败",
		}, nil
	}

	return &pb.SwapDeleteProviderResponse{
		Success: true,
		Message: "删除成功",
	}, nil
}
