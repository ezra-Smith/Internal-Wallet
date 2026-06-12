package logic

import (
	"context"
	"fmt"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/repository"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type ListTransferBatchesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListTransferBatchesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListTransferBatchesLogic {
	return &ListTransferBatchesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListTransferBatchesLogic) ListTransferBatches(in *pb.ListTransferBatchesRequest) (*pb.ListTransferBatchesResponse, error) {
	if in == nil {
		in = &pb.ListTransferBatchesRequest{}
	}
	if l.svcCtx.DB == nil || l.svcCtx.TransferBatchRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	createdFrom, createdTo, err := parseDateFromTo(in.DateFrom, in.DateTo)
	if err != nil {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_DATE", "invalid date", map[string]string{"date_from": "invalid"})
	}

	f := repository.TransferBatchListFilter{
		Status: strings.TrimSpace(in.Status),
		// Transfer batches are internal-ledger only.
		Network:     "INTERNAL",
		Currency:    strings.TrimSpace(in.Currency),
		Keyword:     strings.TrimSpace(in.Keyword),
		CreatedFrom: createdFrom,
		CreatedTo:   createdTo,
	}
	items, total, err := l.svcCtx.TransferBatchRepo.List(l.ctx, in.Page, in.PageSize, f)
	if err != nil {
		l.Logger.Errorf("list transfer batches failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	respItems := make([]*pb.TransferBatchItem, 0, len(items))
	for _, it := range items {
		respItems = append(respItems, toPBTransferBatchItem(it))
	}
	p := calcPagination(in.Page, in.PageSize, total)

	processingCount := int64(0)
	if err := l.svcCtx.DB.WithContext(l.ctx).
		Model(&model.TransferBatchModel{}).
		Where("status = ?", "processing").
		Count(&processingCount).Error; err != nil {
		l.Logger.Errorf("count processing batches failed: %v", err)
	}

	totalTransferredUSD := "0"
	_ = l.svcCtx.DB.WithContext(l.ctx).
		Model(&model.TransferBatchModel{}).
		Select("COALESCE(SUM(total_amount_usd), 0)").
		Where("status = ?", "completed").
		Scan(&totalTransferredUSD).Error

	return &pb.ListTransferBatchesResponse{
		Success: true,
		Message: fmt.Sprintf("ok (%d)", total),
		Data: &pb.ListTransferBatchesData{
			Batches: respItems,
			Summary: &pb.TransferBatchSummary{
				TotalBatches:        total,
				ProcessingBatches:   processingCount,
				TotalTransferredUsd: totalTransferredUSD,
			},
		},
		Pagination: p,
		RequestId:  resp.RequestID(l.ctx),
		Timestamp:  resp.Timestamp(),
	}, nil
}
