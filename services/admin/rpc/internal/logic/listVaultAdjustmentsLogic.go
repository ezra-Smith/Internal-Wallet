package logic

import (
	"context"
	"fmt"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/repository"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type ListVaultAdjustmentsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListVaultAdjustmentsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListVaultAdjustmentsLogic {
	return &ListVaultAdjustmentsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListVaultAdjustmentsLogic) ListVaultAdjustments(in *pb.ListVaultAdjustmentsRequest) (*pb.ListVaultAdjustmentsResponse, error) {
	if in == nil {
		in = &pb.ListVaultAdjustmentsRequest{}
	}
	if l.svcCtx.DB == nil || l.svcCtx.VaultAdjustmentRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	statuses := parseCSV(in.Status)
	for _, s := range statuses {
		if !validateVaultAdjustmentStatus(s) {
			return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_STATUS", "invalid status", map[string]string{"status": "invalid"})
		}
	}

	adjType := strings.TrimSpace(in.AdjustmentType)
	if adjType != "" && !validateVaultAdjustmentType(adjType) {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_ADJUSTMENT_TYPE", "invalid adjustment_type", map[string]string{"adjustment_type": "invalid"})
	}

	from, to, err := parseDateFromTo(in.DateFrom, in.DateTo)
	if err != nil {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_DATE_RANGE", "invalid date range", map[string]string{"date": "invalid"})
	}

	sortBy := strings.TrimSpace(in.SortBy)
	if sortBy != "" && sortBy != "created_at" && sortBy != "amount" && sortBy != "status" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_SORT_BY", "invalid sort_by", map[string]string{"sort_by": "invalid"})
	}
	sortOrder := strings.TrimSpace(in.SortOrder)
	if sortOrder != "" && !strings.EqualFold(sortOrder, "asc") && !strings.EqualFold(sortOrder, "desc") {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_SORT_ORDER", "invalid sort_order", map[string]string{"sort_order": "invalid"})
	}

	items, total, err := l.svcCtx.VaultAdjustmentRepo.List(
		l.ctx,
		in.Page,
		in.PageSize,
		repository.VaultAdjustmentListFilter{
			Statuses:       statuses,
			Network:        strings.TrimSpace(in.Network),
			ChainID:        in.ChainId,
			Currency:       strings.TrimSpace(in.Currency),
			AdjustmentType: adjType,
			DateFrom:       from,
			DateTo:         to,
			SortBy:         sortBy,
			SortOrder:      sortOrder,
		},
	)
	if err != nil {
		l.Logger.Errorf("list vault adjustments failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	respItems := make([]*pb.VaultAdjustmentItem, 0, len(items))
	for _, it := range items {
		respItems = append(respItems, toPBVaultAdjustmentItem(it))
	}

	p := calcPagination(in.Page, in.PageSize, total)
	return &pb.ListVaultAdjustmentsResponse{
		Success: true,
		Message: fmt.Sprintf("ok (%d)", total),
		Data: &pb.ListVaultAdjustmentsData{
			Adjustments: respItems,
		},
		Pagination: p,
		RequestId:  resp.RequestID(l.ctx),
		Timestamp:  resp.Timestamp(),
	}, nil
}
