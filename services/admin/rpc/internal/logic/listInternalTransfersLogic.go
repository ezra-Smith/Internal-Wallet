package logic

import (
	"context"
	"fmt"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	admininterceptor "internalwallet/services/admin/rpc/internal/interceptor"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/repository"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type ListInternalTransfersLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListInternalTransfersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListInternalTransfersLogic {
	return &ListInternalTransfersLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListInternalTransfersLogic) ListInternalTransfers(in *pb.ListInternalTransfersRequest) (*pb.ListInternalTransfersResponse, error) {
	if in == nil {
		in = &pb.ListInternalTransfersRequest{}
	}
	if l.svcCtx.DB == nil || l.svcCtx.CurrencyTransferOrderRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}
	if _, ok := admininterceptor.GetCurrentAdmin(l.ctx); !ok {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	// Parse status filter
	statuses := parseCSV(in.Status)
	for _, s := range statuses {
		if !validateTransferStatus(s) {
			return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_STATUS", "invalid status", map[string]string{"status": "invalid"})
		}
	}

	// Parse strategy filter
	strategies := parseCSV(in.Strategy)
	for _, s := range strategies {
		if !isValidTransferStrategy(s) {
			return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_STRATEGY", "invalid strategy", map[string]string{"strategy": "invalid"})
		}
	}

	// Parse sort parameters
	sortBy := strings.TrimSpace(in.SortBy)
	if sortBy != "" && sortBy != "created_at" && sortBy != "amount" && sortBy != "status" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_SORT_BY", "invalid sort_by", map[string]string{"sort_by": "invalid"})
	}
	sortOrder := strings.TrimSpace(in.SortOrder)
	if sortOrder != "" && !strings.EqualFold(sortOrder, "asc") && !strings.EqualFold(sortOrder, "desc") {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_SORT_ORDER", "invalid sort_order", map[string]string{"sort_order": "invalid"})
	}

	// Parse date range
	createdFrom, createdTo, err := parseDateFromTo(in.CreatedFrom, in.CreatedTo)
	if err != nil {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_DATE", "invalid date", map[string]string{"created_from": "invalid"})
	}

	// Parse amount range
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

	// Convert date range to string format
	createdFromStr := ""
	createdToStr := ""
	if createdFrom != nil {
		createdFromStr = createdFrom.Format("2006-01-02")
	}
	if createdTo != nil {
		createdToStr = createdTo.Format("2006-01-02")
	}

	f := repository.CurrencyTransferOrderListFilter{
		FromUserID:  in.FromUserId,
		ToUserID:    in.ToUserId,
		AssetCode:   in.AssetCode,
		Status:      statuses,
		Strategy:    strategies,
		CreatedFrom: createdFromStr,
		CreatedTo:   createdToStr,
		MinAmount:   minAmt,
		MaxAmount:   maxAmt,
		SortBy:      sortBy,
		SortOrder:   sortOrder,
		Page:        in.Page,
		PageSize:    in.PageSize,
	}

	items, total, err := l.svcCtx.CurrencyTransferOrderRepo.List(l.ctx, f)
	if err != nil {
		l.Logger.Errorf("list internal transfers failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	// Convert to protobuf items with user info
	respItems := make([]*pb.InternalTransferItem, 0, len(items))
	for _, it := range items {
		item := toPBInternalTransferItem(it)
		// Fetch user info if needed
		if l.svcCtx.UserRepo != nil {
			if fromUser, err := l.svcCtx.UserRepo.FindByID(l.ctx, it.FromUserID); err == nil && fromUser != nil {
				item.FromUserEmail = strings.TrimSpace(fromUser.Email)
				item.FromUserPhone = strings.TrimSpace(fromUser.Phone)
			}
			if toUser, err := l.svcCtx.UserRepo.FindByID(l.ctx, it.ToUserID); err == nil && toUser != nil {
				item.ToUserEmail = strings.TrimSpace(toUser.Email)
				item.ToUserPhone = strings.TrimSpace(toUser.Phone)
			}
		}
		respItems = append(respItems, item)
	}

	// 获取统计汇总数据
	var summary *pb.InternalTransferSummary
	if summaryData, err := l.svcCtx.CurrencyTransferOrderRepo.GetSummary(l.ctx); err == nil && summaryData != nil {
		// 处理 TotalAmount，保留6位小数并四舍五入
		totalAmount := summaryData.TotalAmount
		if totalAmountDecimal, err := decimal.NewFromString(summaryData.TotalAmount); err == nil {
			totalAmount = totalAmountDecimal.Round(6).String()
		}

		summary = &pb.InternalTransferSummary{
			Total:       summaryData.Total,
			Pending:     summaryData.Pending,
			Completed:   summaryData.Completed,
			Failed:      summaryData.Failed,
			Cancelled:   summaryData.Cancelled,
			Rejected:    summaryData.Rejected,
			TotalAmount: totalAmount,
		}
	} else if err != nil {
		l.Logger.Infof("get internal transfer summary failed: %v", err)
	}

	p := calcPagination(in.Page, in.PageSize, total)
	return &pb.ListInternalTransfersResponse{
		Success: true,
		Message: fmt.Sprintf("ok (%d)", total),
		Data: &pb.ListInternalTransfersData{
			Transfers: respItems,
			Summary:   summary,
		},
		Pagination: p,
		RequestId:  resp.RequestID(l.ctx),
		Timestamp:  resp.Timestamp(),
	}, nil
}

func validateTransferStatus(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "pending", "processing", "completed", "failed", "cancelled", "rejected":
		return true
	default:
		return false
	}
}

func isValidTransferStrategy(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "auto", "manual_auto":
		return true
	default:
		return false
	}
}

func toPBInternalTransferItem(m *model.CurrencyTransferOrderModel) *pb.InternalTransferItem {
	item := &pb.InternalTransferItem{
		Id:               m.ID,
		FromUserId:       m.FromUserID,
		ToUserId:         m.ToUserID,
		AssetCode:        strings.TrimSpace(m.AssetCode),
		Amount:           strings.TrimSpace(m.Amount),
		Fee:              strings.TrimSpace(m.Fee),
		Strategy:         strings.TrimSpace(m.Strategy),
		Status:           strings.TrimSpace(m.Status),
		AuditAdminId:     m.AuditAdminID,
		UpdatedBy:        m.UpdatedBy,
		CreatedByAdminId: m.CreatedByAdmin,
		CreatedAt:        formatTime(m.CreatedAt),
		UpdatedAt:        formatTime(m.UpdatedAt),
	}
	if m.Note != nil {
		item.Note = strings.TrimSpace(*m.Note)
	}
	if m.ErrorMessage != nil {
		item.ErrorMessage = strings.TrimSpace(*m.ErrorMessage)
	}
	if m.LedgerTxID != nil {
		item.LedgerTxId = *m.LedgerTxID
	}
	if m.AuditNote != nil {
		item.AuditNote = strings.TrimSpace(*m.AuditNote)
	}
	if m.AuditedAt != nil {
		item.AuditedAt = formatTime(*m.AuditedAt)
	}
	return item
}
