package logic

import (
	"context"
	"fmt"
	"strings"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/repository"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type ListWeb3UserTransactionsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListWeb3UserTransactionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListWeb3UserTransactionsLogic {
	return &ListWeb3UserTransactionsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListWeb3UserTransactionsLogic) ListWeb3UserTransactions(in *pb.ListWeb3UserTransactionsRequest) (*pb.ListWeb3UserTransactionsResponse, error) {
	deviceID := strings.TrimSpace(in.DeviceId)
	if deviceID == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "device_id required", map[string]string{"device_id": "required"})
	}

	if l.svcCtx.DB == nil || l.svcCtx.Web3UserRepo == nil || l.svcCtx.Web3BalanceChangeRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	// Find user by device_id
	user, err := l.svcCtx.Web3UserRepo.FindByDeviceID(l.ctx, deviceID)
	if err != nil || user == nil {
		return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "DEVICE_NOT_FOUND", "device not found", nil)
	}

	page := in.Page
	if page <= 0 {
		page = 1
	}
	pageSize := in.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	// Build filter
	filter := repository.Web3BalanceChangeFilter{
		TxType:    strings.TrimSpace(in.TxType),
		Direction: strings.TrimSpace(in.Direction),
		AssetCode: strings.TrimSpace(in.AssetCode),
		Status:    strings.TrimSpace(in.Status),
		ChainCode: strings.TrimSpace(in.Network), // Network maps to ChainCode
		SortBy:    strings.TrimSpace(in.SortBy),
		SortOrder: strings.TrimSpace(in.SortOrder),
	}

	// Parse date filters
	if dateFrom := strings.TrimSpace(in.DateFrom); dateFrom != "" {
		if t, err := time.Parse("2006-01-02", dateFrom); err == nil {
			filter.DateFrom = &t
		}
	}
	if dateTo := strings.TrimSpace(in.DateTo); dateTo != "" {
		if t, err := time.Parse("2006-01-02", dateTo); err == nil {
			// Set to end of day
			t = t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
			filter.DateTo = &t
		}
	}

	// Query balance changes by user ID (all addresses)
	txs, total, err := l.svcCtx.Web3BalanceChangeRepo.ListByWeb3UserID(l.ctx, user.ID, page, pageSize, filter)
	if err != nil {
		l.Logger.Errorf("list balance changes failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "query failed", nil)
	}

	// Convert to response items
	items := make([]*pb.AdminWeb3TransactionItem, 0, len(txs))
	for _, tx := range txs {
		if tx == nil {
			continue
		}

		item := &pb.AdminWeb3TransactionItem{
			Id:            fmt.Sprintf("bc-%d", tx.ID),
			TxHash:        tx.TxHash,
			Network:       tx.ChainCode,
			ChainId:       tx.ChainID,
			UserAddress:   tx.UserAddress,
			TxType:        tx.TxType,
			Direction:     tx.Direction,
			AssetCode:     tx.AssetCode,
			Amount:        tx.Amount,
			AmountUsd:     "", // web3_balance_changes doesn't have amount_usd field
			Status:        tx.Status,
			Confirmations: tx.Confirmations,
			BlockTime:     formatTimePtr(tx.BlockTime),
			CreatedAt:     formatTime(tx.CreatedAt),
		}

		// Handle nullable fields (AmountUsd removed as web3_balance_changes doesn't have it)
		if tx.FromAddress != nil {
			item.FromAddress = *tx.FromAddress
		}
		if tx.ToAddress != nil {
			item.ToAddress = *tx.ToAddress
		}
		if tx.Fee != nil {
			item.Fee = *tx.Fee
		}
		if tx.FeeAsset != nil {
			item.FeeAsset = *tx.FeeAsset
		}
		if tx.BlockNumber != nil {
			item.BlockNumber = *tx.BlockNumber
		}

		items = append(items, item)
	}

	totalPages := int32((total + int64(pageSize) - 1) / int64(pageSize))

	return &pb.ListWeb3UserTransactionsResponse{
		Success: true,
		Message: "ok",
		Data: &pb.ListWeb3UserTransactionsData{
			Items: items,
			Pagination: &pb.Pagination{
				Page:       page,
				PageSize:   pageSize,
				Total:      total,
				TotalPages: totalPages,
			},
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
