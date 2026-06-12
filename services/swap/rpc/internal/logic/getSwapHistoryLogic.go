package logic

import (
	"context"
	"fmt"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/swap/rpc/internal/repository"
	"internalwallet/services/swap/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetSwapHistoryLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetSwapHistoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSwapHistoryLogic {
	return &GetSwapHistoryLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetSwapHistoryLogic) GetSwapHistory(in *pb.GetSwapHistoryRequest) (*pb.GetSwapHistoryResponse, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if !isValidWalletAddress(in.WalletAddress) {
		return nil, status.Error(codes.InvalidArgument, "invalid wallet_address")
	}
	if l.svcCtx.DB == nil || l.svcCtx.TxRepo == nil {
		return nil, status.Error(codes.Internal, "db not configured")
	}

	page := in.Page
	pageSize := in.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	filter := repository.SwapTransactionListFilter{
		ChainID: in.ChainId,
	}
	wallet := normalizeEVMAddress(in.WalletAddress)
	items, total, err := l.svcCtx.TxRepo.ListByWallet(l.ctx, wallet, page, pageSize, filter)
	if err != nil {
		l.Logger.Errorw("list swap history failed", logx.Field("error", err))
		return nil, status.Error(codes.Internal, "failed to query swap history")
	}

	out := make([]*pb.SwapHistoryItem, 0, len(items))
	for _, it := range items {
		if it == nil {
			continue
		}
		out = append(out, &pb.SwapHistoryItem{
			SwapId:           fmt.Sprintf("%d", it.ID),
			ChainId:          it.ChainID,
			Provider:         strings.TrimSpace(it.Provider),
			WalletAddress:    it.WalletAddress,
			FromTokenAddress: it.FromToken,
			ToTokenAddress:   it.ToToken,
			FromAmount:       it.FromAmount,
			ToAmount:         it.ToAmount,
			TxHash:           it.TxHash,
			Status:           mapStatusToProto(it.Status),
			CreatedAt:        formatRFC3339(it.CreatedAt),
			UpdatedAt:        formatRFC3339(it.UpdatedAt),
		})
	}

	return &pb.GetSwapHistoryResponse{
		Success:    true,
		Message:    "ok",
		Items:      out,
		Pagination: buildPagination(page, pageSize, total),
	}, nil
}
