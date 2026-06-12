package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateSwapApiKeyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateSwapApiKeyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateSwapApiKeyLogic {
	return &CreateSwapApiKeyLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CreateSwapApiKey 创建Swap API密钥（门面，调用Swap服务）
func (l *CreateSwapApiKeyLogic) CreateSwapApiKey(in *pb.AdminCreateSwapApiKeyRequest) (*pb.AdminCreateSwapApiKeyResponse, error) {
	if l.svcCtx.SwapRpc == nil {
		l.Logger.Errorf("Swap service client is not available")
		return &pb.AdminCreateSwapApiKeyResponse{
			Success: false,
			Message: "Swap服务未配置或未启动",
		}, nil
	}

	// 调用 Swap 服务
	swapReq := &pb.CreateSwapApiKeyRequest{
		ProjectName:   in.ProjectName,
		KeyName:       in.KeyName,
		RateLimitTier: in.RateLimitTier,
		ExpiresAtUnix: in.ExpiresAtUnix,
	}

	resp, err := l.svcCtx.SwapRpc.CreateSwapApiKey(l.ctx, swapReq)
	if err != nil {
		l.Logger.Errorf("Failed to call swap.CreateSwapApiKey: %v", err)
		return &pb.AdminCreateSwapApiKeyResponse{
			Success: false,
			Message: "调用Swap服务失败",
		}, nil
	}

	// 映射响应
	var adminItem *pb.AdminSwapApiKeyItem
	if resp.Item != nil {
		adminItem = &pb.AdminSwapApiKeyItem{
			Id:            resp.Item.Id,
			ProjectName:   resp.Item.ProjectName,
			KeyName:       resp.Item.KeyName,
			RateLimitTier: resp.Item.RateLimitTier,
			IsActive:      resp.Item.IsActive,
			CreatedAtUnix: resp.Item.CreatedAtUnix,
			ExpiresAtUnix: resp.Item.ExpiresAtUnix,
			TokenDisplay:  resp.Item.TokenDisplay,
		}
	}

	return &pb.AdminCreateSwapApiKeyResponse{
		Success: resp.Success,
		Message: resp.Message,
		ApiKey:  resp.ApiKey,
		Item:    adminItem,
	}, nil
}
