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

type ListDepositsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListDepositsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListDepositsLogic {
	return &ListDepositsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListDepositsLogic) ListDeposits(in *pb.ListDepositsRequest) (*pb.ListDepositsResponse, error) {
	if in == nil {
		in = &pb.ListDepositsRequest{}
	}
	if l.svcCtx.DB == nil || l.svcCtx.WalletDepositRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	statuses := parseCSV(in.Status)
	for _, s := range statuses {
		if !validateDepositStatus(s) {
			return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_STATUS", "invalid status", map[string]string{"status": "invalid"})
		}
	}

	sortBy := strings.TrimSpace(in.SortBy)
	if sortBy != "" && sortBy != "created_at" && sortBy != "amount" && sortBy != "status" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_SORT_BY", "invalid sort_by", map[string]string{"sort_by": "invalid"})
	}
	sortOrder := strings.TrimSpace(in.SortOrder)
	if sortOrder != "" && !strings.EqualFold(sortOrder, "asc") && !strings.EqualFold(sortOrder, "desc") {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_SORT_ORDER", "invalid sort_order", map[string]string{"sort_order": "invalid"})
	}

	createdFrom, createdTo, err := parseDateFromTo(in.CreatedFrom, in.CreatedTo)
	if err != nil {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_DATE", "invalid date", map[string]string{"created_from": "invalid"})
	}

	var minAmount, maxAmount *string
	if strings.TrimSpace(in.MinAmount) != "" {
		d, ok := parseNonNegativeDecimal(in.MinAmount)
		if !ok || d.Exponent() < -30 {
			return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_MIN_AMOUNT", "invalid min_amount", map[string]string{"min_amount": "invalid"})
		}
		s := d.String()
		minAmount = &s
	}
	if strings.TrimSpace(in.MaxAmount) != "" {
		d, ok := parseNonNegativeDecimal(in.MaxAmount)
		if !ok || d.Exponent() < -30 {
			return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_MAX_AMOUNT", "invalid max_amount", map[string]string{"max_amount": "invalid"})
		}
		s := d.String()
		maxAmount = &s
	}
	if minAmount != nil && maxAmount != nil {
		minD, _ := parseNonNegativeDecimal(*minAmount)
		maxD, _ := parseNonNegativeDecimal(*maxAmount)
		if maxD.LessThan(minD) {
			return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_AMOUNT_RANGE", "invalid amount range", map[string]string{"max_amount": "lt_min_amount"})
		}
	}

	f := repository.WalletDepositListFilter{
		ID:              in.Id,
		UserID:          in.UserId,
		AssetCode:       normalizeCode(in.AssetCode),
		ChainCode:       normalizeCode(in.ChainCode),
		Statuses:        statuses,
		DepositAddress:  in.DepositAddress,
		TransactionHash: in.TransactionHash,
		CreatedFrom:     createdFrom,
		CreatedTo:       createdTo,
		MinAmount:       minAmount,
		MaxAmount:       maxAmount,
		SortBy:          sortBy,
		SortOrder:       sortOrder,
	}

	items, total, err := l.svcCtx.WalletDepositRepo.List(l.ctx, in.Page, in.PageSize, f)
	if err != nil {
		l.Logger.Errorf("list deposits failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	respItems := make([]*pb.WalletDepositItem, 0, len(items))
	for _, it := range items {
		respItems = append(respItems, toPBDepositItem(it))
	}
	p := calcPagination(in.Page, in.PageSize, total)
	return &pb.ListDepositsResponse{
		Success: true,
		Message: fmt.Sprintf("ok (%d)", total),
		Data: &pb.ListDepositsData{
			Deposits: respItems,
		},
		Pagination: p,
		RequestId:  resp.RequestID(l.ctx),
		Timestamp:  resp.Timestamp(),
	}, nil
}
