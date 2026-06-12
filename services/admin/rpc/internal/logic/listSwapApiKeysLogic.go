package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListSwapApiKeysLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListSwapApiKeysLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListSwapApiKeysLogic {
	return &ListSwapApiKeysLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListSwapApiKeys 列出Swap API密钥（门面，调用Swap服务）
func (l *ListSwapApiKeysLogic) ListSwapApiKeys(in *pb.AdminListSwapApiKeysRequest) (*pb.AdminListSwapApiKeysResponse, error) {
	if l.svcCtx.SwapRpc == nil {
		l.Logger.Errorf("Swap service client is not available")
		return &pb.AdminListSwapApiKeysResponse{
			Success: false,
			Message: "Swap服务未配置或未启动",
		}, nil
	}

	// 调用 Swap 服务
	swapReq := &pb.ListSwapApiKeysRequest{
		Page:     in.Page,
		PageSize: in.PageSize,
	}

	resp, err := l.svcCtx.SwapRpc.ListSwapApiKeys(l.ctx, swapReq)
	if err != nil {
		l.Logger.Errorf("Failed to call swap.ListSwapApiKeys: %v", err)
		return &pb.AdminListSwapApiKeysResponse{
			Success: false,
			Message: "调用Swap服务失败",
		}, nil
	}

	// 映射响应
	var adminItems []*pb.AdminSwapApiKeyItem
	for _, item := range resp.Items {
		adminItems = append(adminItems, &pb.AdminSwapApiKeyItem{
			Id:            item.Id,
			ProjectName:   item.ProjectName,
			KeyName:       item.KeyName,
			RateLimitTier: item.RateLimitTier,
			IsActive:      item.IsActive,
			CreatedAtUnix: item.CreatedAtUnix,
			ExpiresAtUnix: item.ExpiresAtUnix,
			TokenDisplay:  item.TokenDisplay,
		})
	}

	// 映射分页信息
	var adminPagination *pb.Pagination
	if resp.Pagination != nil {
		adminPagination = &pb.Pagination{
			Page:       resp.Pagination.Page,
			PageSize:   resp.Pagination.PageSize,
			Total:      resp.Pagination.Total,
			TotalPages: resp.Pagination.TotalPages,
		}
	}

	return &pb.AdminListSwapApiKeysResponse{
		Success:    resp.Success,
		Message:    resp.Message,
		Items:      adminItems,
		Pagination: adminPagination,
	}, nil
}
