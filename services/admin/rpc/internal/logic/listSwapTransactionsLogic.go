package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListSwapTransactionsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListSwapTransactionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListSwapTransactionsLogic {
	return &ListSwapTransactionsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListSwapTransactionsLogic) ListSwapTransactions(in *pb.AdminListSwapTransactionsRequest) (*pb.AdminListSwapTransactionsResponse, error) {
	if l.svcCtx.SwapRpc == nil {
		l.Logger.Errorf("Swap service client is not available")
		return &pb.AdminListSwapTransactionsResponse{
			Success: false,
			Message: "Swap服务未配置或未启动",
		}, nil
	}

	resp, err := l.svcCtx.SwapRpc.ListSwapTransactions(l.ctx, &pb.SwapListSwapTransactionsRequest{
		WalletAddress: in.WalletAddress,
		ChainId:       in.ChainId,
		Provider:      in.Provider,
		Status:        in.Status,
		StartDate:     in.StartDate,
		EndDate:       in.EndDate,
		Page:          in.Page,
		PageSize:      in.PageSize,
	})
	if err != nil {
		l.Logger.Errorf("Failed to call swap.ListSwapTransactions: %v", err)
		return &pb.AdminListSwapTransactionsResponse{
			Success: false,
			Message: "调用Swap服务失败",
		}, nil
	}

	return &pb.AdminListSwapTransactionsResponse{
		Success:    resp.Success,
		Message:    resp.Message,
		Items:      mapSwapTransactionItems(resp.Items),
		Pagination: mapSwapPagination(resp.Pagination),
	}, nil
}
