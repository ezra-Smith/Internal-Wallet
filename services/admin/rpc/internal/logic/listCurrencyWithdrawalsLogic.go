package logic

import (
	"context"
	"fmt"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	admininterceptor "internalwallet/services/admin/rpc/internal/interceptor"
	"internalwallet/services/admin/rpc/internal/repository"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type ListCurrencyWithdrawalsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListCurrencyWithdrawalsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListCurrencyWithdrawalsLogic {
	return &ListCurrencyWithdrawalsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListCurrencyWithdrawalsLogic) ListCurrencyWithdrawals(in *pb.ListCurrencyWithdrawalsRequest) (*pb.ListCurrencyWithdrawalsResponse, error) {
	if in == nil {
		in = &pb.ListCurrencyWithdrawalsRequest{}
	}
	if l.svcCtx.DB == nil || l.svcCtx.CurrencyWithdrawOrderRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}
	if _, ok := admininterceptor.GetCurrentAdmin(l.ctx); !ok {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	statuses := parseCSV(in.Status)
	for _, s := range statuses {
		if !validateWithdrawalStatus(s) {
			return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_STATUS", "invalid status", map[string]string{"status": "invalid"})
		}
	}
	strategies := parseCSV(in.Strategy)
	for _, s := range strategies {
		if !isValidWithdrawAuditStrategy(s) {
			return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_STRATEGY", "invalid strategy", map[string]string{"strategy": "invalid"})
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

	var minAmt *string
	if strings.TrimSpace(in.MinAmount) != "" {
		d, ok := parseNonNegativeDecimal(strings.TrimSpace(in.MinAmount))
		if !ok {
			return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_MIN_AMOUNT", "invalid min_amount", map[string]string{"min_amount": "invalid"})
		}
		ss := d.String()
		minAmt = &ss
	}
	var maxAmt *string
	if strings.TrimSpace(in.MaxAmount) != "" {
		d, ok := parseNonNegativeDecimal(strings.TrimSpace(in.MaxAmount))
		if !ok {
			return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_MAX_AMOUNT", "invalid max_amount", map[string]string{"max_amount": "invalid"})
		}
		ss := d.String()
		maxAmt = &ss
	}
	if minAmt != nil && maxAmt != nil {
		minD, _ := decimal.NewFromString(*minAmt)
		maxD, _ := decimal.NewFromString(*maxAmt)
		if maxD.LessThan(minD) {
			return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_AMOUNT_RANGE", "invalid amount range", map[string]string{"max_amount": "lt_min_amount"})
		}
	}

	f := repository.CurrencyWithdrawOrderListFilter{
		UserID:      in.UserId,
		AssetCode:   in.AssetCode,
		ChainCode:   in.ChainCode,
		Statuses:    statuses,
		Strategies:  strategies,
		ToAddress:   in.ToAddress,
		TxHash:      in.TxHash,
		CreatedFrom: createdFrom,
		CreatedTo:   createdTo,
		MinAmount:   minAmt,
		MaxAmount:   maxAmt,
		SortBy:      sortBy,
		SortOrder:   sortOrder,
	}

	items, total, err := l.svcCtx.CurrencyWithdrawOrderRepo.List(l.ctx, in.Page, in.PageSize, f)
	if err != nil {
		l.Logger.Errorf("list currency withdrawals failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	respItems := make([]*pb.CurrencyWithdrawalItem, 0, len(items))
	for _, it := range items {
		respItems = append(respItems, toPBCurrencyWithdrawalItem(it))
	}
	p := calcPagination(in.Page, in.PageSize, total)
	return &pb.ListCurrencyWithdrawalsResponse{
		Success: true,
		Message: fmt.Sprintf("ok (%d)", total),
		Data: &pb.ListCurrencyWithdrawalsData{
			Withdrawals: respItems,
		},
		Pagination: p,
		RequestId:  resp.RequestID(l.ctx),
		Timestamp:  resp.Timestamp(),
	}, nil

}
