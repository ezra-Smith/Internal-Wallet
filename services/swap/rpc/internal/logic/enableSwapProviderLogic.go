package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/swap/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type EnableSwapProviderLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewEnableSwapProviderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EnableSwapProviderLogic {
	return &EnableSwapProviderLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// EnableSwapProvider 启用Swap服务商
func (l *EnableSwapProviderLogic) EnableSwapProvider(in *pb.SwapEnableProviderRequest) (*pb.SwapEnableProviderResponse, error) {
	// 参数验证
	if in.Id <= 0 {
		return &pb.SwapEnableProviderResponse{
			Success: false,
			Message: "无效的ID",
		}, nil
	}

	// 检查是否存在
	existing, err := l.svcCtx.SwapProviderRepo.GetByID(l.ctx, in.Id)
	if err != nil {
		l.Logger.Errorf("Failed to get provider: %v", err)
		return &pb.SwapEnableProviderResponse{
			Success: false,
			Message: "服务商不存在",
		}, nil
	}
	if existing == nil {
		return &pb.SwapEnableProviderResponse{
			Success: false,
			Message: "服务商不存在",
		}, nil
	}

	// 启用
	if err := l.svcCtx.SwapProviderRepo.Enable(l.ctx, in.Id); err != nil {
		l.Logger.Errorf("Failed to enable swap provider: %v", err)
		return &pb.SwapEnableProviderResponse{
			Success: false,
			Message: "启用服务商失败",
		}, nil
	}

	return &pb.SwapEnableProviderResponse{
		Success: true,
		Message: "启用成功",
	}, nil
}
