package logic

import (
	"context"
	"fmt"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/swap/rpc/internal/interceptor"
	"internalwallet/services/swap/rpc/internal/repository"
	"internalwallet/services/swap/rpc/internal/svc"

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

func (l *ListSwapTransactionsLogic) ListSwapTransactions(in *pb.SwapListSwapTransactionsRequest) (*pb.SwapListSwapTransactionsResponse, error) {
	if l.svcCtx.TxRepo == nil {
		return &pb.SwapListSwapTransactionsResponse{
			Success: false,
			Message: "transaction repository not configured",
		}, nil
	}

	// Get project name from authenticated context
	projectName := interceptor.GetProjectName(l.ctx)

	page := in.Page
	if page <= 0 {
		page = 1
	}
	pageSize := in.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}

	filter := repository.SwapTransactionListFilter{
		ChainID:     in.ChainId,
		Provider:    in.Provider,
		Status:      in.Status,
		ProjectName: projectName,
		StartDate:   in.StartDate,
		EndDate:     in.EndDate,
	}

	items, total, err := l.svcCtx.TxRepo.ListWithFilters(l.ctx, projectName, page, pageSize, filter)
	if err != nil {
		logx.Errorf("List transactions failed: %v", err)
		return &pb.SwapListSwapTransactionsResponse{
			Success: false,
			Message: fmt.Sprintf("query failed: %v", err),
		}, nil
	}

	var transactions []*pb.SwapTransactionItem
	for _, item := range items {
		transactions = append(transactions, &pb.SwapTransactionItem{
			Id:                item.ID,
			ProjectName:       item.ProjectName,
			WalletAddress:     item.WalletAddress,
			ChainId:           item.ChainID,
			ChainName:         fmt.Sprintf("Chain %d", item.ChainID),
			Provider:          item.Provider,
			FromTokenSymbol:   item.FromToken,
			FromTokenAddress:  item.FromToken,
			FromTokenDecimals: 18,
			ToTokenSymbol:     item.ToToken,
			ToTokenAddress:    item.ToToken,
			ToTokenDecimals:   18,
			FromAmount:        item.FromAmount,
			ToAmount:          item.ToAmount,
			TxHash:            item.TxHash,
			Status:            item.Status,
			FeeUsd:            "0",
			Gas:               item.TxGas,
			CreatedAt:         item.CreatedAt.Format(time.RFC3339),
			BroadcastedAt:     "",
		})
		if item.BroadcastedAt != nil {
			transactions[len(transactions)-1].BroadcastedAt = item.BroadcastedAt.Format(time.RFC3339)
		}
	}

	totalPages := int32((total + int64(pageSize) - 1) / int64(pageSize))
	return &pb.SwapListSwapTransactionsResponse{
		Success: true,
		Message: "success",
		Items:   transactions,
		Pagination: &pb.SwapPagination{
			Page:       page,
			PageSize:   pageSize,
			Total:      total,
			TotalPages: totalPages,
			HasNext:    page < totalPages,
			HasPrev:    page > 1,
		},
	}, nil
}
