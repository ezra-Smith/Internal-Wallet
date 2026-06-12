package logic

import (
	"context"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type SwapGetSwapHistoryLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSwapGetSwapHistoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SwapGetSwapHistoryLogic {
	return &SwapGetSwapHistoryLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SwapGetSwapHistoryLogic) SwapGetSwapHistory(in *pb.BusinessSwapGetSwapHistoryRequest) (*pb.BusinessSwapGetSwapHistoryResponse, error) {
	if in == nil {
		return nil, errx.InvalidParam("invalid params")
	}
	if l.svcCtx.SwapRpc == nil {
		return nil, errx.SwapServiceNotAvailable()
	}
	if strings.TrimSpace(in.WalletAddress) == "" {
		return nil, errx.InvalidWalletAddress()
	}
	page := in.Page
	if page <= 0 {
		page = 1
	}
	pageSize := in.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}

	resp, err := l.svcCtx.SwapRpc.GetSwapHistory(l.ctx, &pb.GetSwapHistoryRequest{
		WalletAddress: in.WalletAddress,
		ChainId:       in.ChainId,
		Page:          page,
		PageSize:      pageSize,
	})
	if err != nil {
		l.Errorf("swap get history failed: %v", err)
		return nil, errx.SwapServiceError()
	}
	if resp == nil {
		return nil, errx.SwapServiceError()
	}
	if !resp.Success {
		return nil, errx.Internal(resp.Message)
	}

	items := make([]*pb.BusinessSwapHistoryItem, 0, len(resp.Items))
	for _, it := range resp.Items {
		items = append(items, mapSwapHistoryItemToBiz(it))
	}
	return &pb.BusinessSwapGetSwapHistoryResponse{
		Success:    true,
		Message:    "ok",
		Items:      items,
		Pagination: mapSwapPaginationToBiz(resp.Pagination),
	}, nil
}
