package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/swap/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type DisableSwapProviderLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDisableSwapProviderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DisableSwapProviderLogic {
	return &DisableSwapProviderLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// DisableSwapProvider 禁用Swap服务商
func (l *DisableSwapProviderLogic) DisableSwapProvider(in *pb.SwapDisableProviderRequest) (*pb.SwapDisableProviderResponse, error) {
	// 参数验证
	if in.Id <= 0 {
		return &pb.SwapDisableProviderResponse{
			Success: false,
			Message: "无效的ID",
		}, nil
	}

	// 检查是否存在
	existing, err := l.svcCtx.SwapProviderRepo.GetByID(l.ctx, in.Id)
	if err != nil {
		l.Logger.Errorf("Failed to get provider: %v", err)
		return &pb.SwapDisableProviderResponse{
			Success: false,
			Message: "服务商不存在",
		}, nil
	}
	if existing == nil {
		return &pb.SwapDisableProviderResponse{
			Success: false,
			Message: "服务商不存在",
		}, nil
	}

	// 禁用
	if err := l.svcCtx.SwapProviderRepo.Disable(l.ctx, in.Id); err != nil {
		l.Logger.Errorf("Failed to disable swap provider: %v", err)
		return &pb.SwapDisableProviderResponse{
			Success: false,
			Message: "禁用服务商失败",
		}, nil
	}

	return &pb.SwapDisableProviderResponse{
		Success: true,
		Message: "禁用成功",
	}, nil
}
