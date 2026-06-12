package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type RevokeSwapApiKeyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRevokeSwapApiKeyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RevokeSwapApiKeyLogic {
	return &RevokeSwapApiKeyLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// RevokeSwapApiKey 撤销Swap API密钥（门面，调用Swap服务）
func (l *RevokeSwapApiKeyLogic) RevokeSwapApiKey(in *pb.AdminRevokeSwapApiKeyRequest) (*pb.AdminRevokeSwapApiKeyResponse, error) {
	if l.svcCtx.SwapRpc == nil {
		l.Logger.Errorf("Swap service client is not available")
		return &pb.AdminRevokeSwapApiKeyResponse{
			Success: false,
			Message: "Swap服务未配置或未启动",
		}, nil
	}

	// 调用 Swap 服务
	swapReq := &pb.RevokeSwapApiKeyRequest{
		Id: in.Id,
	}

	resp, err := l.svcCtx.SwapRpc.RevokeSwapApiKey(l.ctx, swapReq)
	if err != nil {
		l.Logger.Errorf("Failed to call swap.RevokeSwapApiKey: %v", err)
		return &pb.AdminRevokeSwapApiKeyResponse{
			Success: false,
			Message: "调用Swap服务失败",
		}, nil
	}

	return &pb.AdminRevokeSwapApiKeyResponse{
		Success: resp.Success,
		Message: resp.Message,
	}, nil
}
