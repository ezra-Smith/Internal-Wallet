package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/repository"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type GetTransferBatchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetTransferBatchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetTransferBatchLogic {
	return &GetTransferBatchLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetTransferBatchLogic) GetTransferBatch(in *pb.GetTransferBatchRequest) (*pb.GetTransferBatchResponse, error) {
	if in == nil || strings.TrimSpace(in.BatchId) == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "batch_id required", map[string]string{"batch_id": "required"})
	}
	if l.svcCtx.DB == nil || l.svcCtx.TransferBatchRepo == nil || l.svcCtx.TransferBatchItemRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	batch, err := l.svcCtx.TransferBatchRepo.FindByID(l.ctx, in.BatchId)
	if err != nil || batch == nil {
		return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "BATCH_NOT_FOUND", "batch not found", nil)
	}

	statusCounts, err := l.svcCtx.TransferBatchItemRepo.CountByBatchGroupByStatus(l.ctx, in.BatchId)
	if err != nil {
		l.Logger.Errorf("count batch items failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}
	successCnt := int32(statusCounts["success"])
	failedCnt := int32(statusCounts["failed"])
	pendingCnt := int32(statusCounts["pending"] + statusCounts["retrying"] + statusCounts["processing"])

	// Check if any filters are provided
	hasFilters := strings.TrimSpace(in.Id) != "" ||
		strings.TrimSpace(in.Uid) != "" ||
		strings.TrimSpace(in.Nickname) != "" ||
		strings.TrimSpace(in.Email) != "" ||
		strings.TrimSpace(in.Phone) != "" ||
		strings.TrimSpace(in.Role) != "" ||
		strings.TrimSpace(in.Currency) != "" ||
		strings.TrimSpace(in.Amount) != "" ||
		strings.TrimSpace(in.Note) != "" ||
		strings.TrimSpace(in.ErrorMessage) != "" ||
		strings.TrimSpace(in.ProcessedAtStart) != "" ||
		strings.TrimSpace(in.ProcessedAtEnd) != "" ||
		strings.TrimSpace(in.Status) != ""

	var items []*model.TransferBatchItemModel
	var total int64

	if hasFilters {
		// Use filtered query with all conditions
		filters := &repository.TransferBatchItemFilters{
			ID:              strings.TrimSpace(in.Id),
			UID:             strings.TrimSpace(in.Uid),
			Nickname:        strings.TrimSpace(in.Nickname),
			Email:           strings.TrimSpace(in.Email),
			Phone:           strings.TrimSpace(in.Phone),
			Role:            strings.TrimSpace(in.Role),
			Currency:        strings.TrimSpace(in.Currency),
			Amount:          strings.TrimSpace(in.Amount),
			Note:            strings.TrimSpace(in.Note),
			ErrorMessage:    strings.TrimSpace(in.ErrorMessage),
			ProcessedAtFrom: strings.TrimSpace(in.ProcessedAtStart),
			ProcessedAtTo:   strings.TrimSpace(in.ProcessedAtEnd),
			Status:          strings.TrimSpace(in.Status),
		}
		items, total, err = l.svcCtx.TransferBatchItemRepo.ListByBatchWithFilters(l.ctx, in.BatchId, in.Page, in.PageSize, filters)
	} else {
		// Use simple query for backward compatibility
		items, total, err = l.svcCtx.TransferBatchItemRepo.ListByBatch(l.ctx, in.BatchId, in.Page, in.PageSize, strings.TrimSpace(in.Status))
	}

	if err != nil {
		l.Logger.Errorf("list batch items failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	// Currency summary (full batch, not paged).
	type currencySummaryRow struct {
		Currency        string `gorm:"column:currency"`
		TotalRecipients int32  `gorm:"column:total_recipients"`
		TotalAmount     string `gorm:"column:total_amount"`
	}
	var currencyRows []currencySummaryRow
	if err := l.svcCtx.DB.WithContext(l.ctx).
		Table("transfer_batch_items i").
		Joins("JOIN transfer_batches b ON b.batch_id COLLATE utf8mb4_general_ci = i.batch_id COLLATE utf8mb4_general_ci").
		Select("COALESCE(NULLIF(i.currency, ''), b.currency) AS currency, COUNT(1) AS total_recipients, COALESCE(SUM(i.amount), 0) AS total_amount").
		Where("i.batch_id = ?", strings.TrimSpace(in.BatchId)).
		Group("COALESCE(NULLIF(i.currency, ''), b.currency)").
		Order("currency ASC").
		Scan(&currencyRows).Error; err != nil {
		l.Logger.Errorf("get batch currency summary failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}
	respCurrencySummary := make([]*pb.TransferBatchCurrencySummary, 0, len(currencyRows))
	for _, r := range currencyRows {
		ccy := strings.TrimSpace(r.Currency)
		if ccy == "" {
			continue
		}
		respCurrencySummary = append(respCurrencySummary, &pb.TransferBatchCurrencySummary{
			Currency:        ccy,
			TotalRecipients: r.TotalRecipients,
			TotalAmount:     strings.TrimSpace(r.TotalAmount),
		})
	}

	// Enrich recipients with user info (paged).
	type userRow struct {
		ID          int64  `gorm:"column:id"`
		Email       string `gorm:"column:email"`
		Phone       string `gorm:"column:phone"`
		CountryCode string `gorm:"column:country_code"`
		Nickname    string `gorm:"column:nickname"`
		MemberLevel int32  `gorm:"column:member_level"`
	}
	userIDs := make([]int64, 0, len(items))
	seenUserID := map[int64]struct{}{}
	itemUserID := make([]int64, 0, len(items))
	for _, it := range items {
		var uid int64
		if it != nil && it.UserID != nil && *it.UserID > 0 {
			uid = *it.UserID
		} else if it != nil {
			if id, err := strconv.ParseInt(strings.TrimSpace(it.ToAddress), 10, 64); err == nil && id > 0 {
				uid = id
			}
		}
		itemUserID = append(itemUserID, uid)
		if uid <= 0 {
			continue
		}
		if _, ok := seenUserID[uid]; ok {
			continue
		}
		seenUserID[uid] = struct{}{}
		userIDs = append(userIDs, uid)
	}

	userByID := map[int64]userRow{}
	if len(userIDs) > 0 {
		var users []userRow
		if err := l.svcCtx.DB.WithContext(l.ctx).
			Table("users").
			Select("id, email, phone, country_code, nickname, member_level").
			Where("id IN ? AND deleted_at IS NULL", userIDs).
			Scan(&users).Error; err != nil {
			l.Logger.Errorf("get batch user info failed: %v", err)
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
		}
		for _, u := range users {
			if u.ID <= 0 {
				continue
			}
			userByID[u.ID] = u
		}
	}

	respItems := make([]*pb.TransferRecipientItem, 0, len(items))
	for idx, it := range items {
		pbItem := toPBTransferRecipientItem(it)
		if pbItem == nil {
			pbItem = &pb.TransferRecipientItem{}
		}
		uid := int64(0)
		if idx < len(itemUserID) {
			uid = itemUserID[idx]
		}
		if uid > 0 {
			pbItem.Uid = fmt.Sprintf("%d", uid)
			if u, ok := userByID[uid]; ok {
				pbItem.Email = strings.TrimSpace(u.Email)
				pbItem.Phone = maskPhone(fullPhone(u.CountryCode, u.Phone))
				pbItem.Nickname = strings.TrimSpace(u.Nickname)
				pbItem.Role = userRoleFromMemberLevel(u.MemberLevel)
			}
		}
		respItems = append(respItems, pbItem)
	}

	p := calcPagination(in.Page, in.PageSize, total)

	pbBatch := toPBTransferBatchItem(batch)
	noteByStatus, extraTimeline := l.getTransferBatchTimelineNotesAndExtra(strings.TrimSpace(in.BatchId))
	timeline := buildTransferBatchTimeline(pbBatch, noteByStatus)
	if len(extraTimeline) > 0 {
		timeline = append(timeline, extraTimeline...)
		sortTransferBatchTimeline(timeline)
	}

	return &pb.GetTransferBatchResponse{
		Success: true,
		Message: fmt.Sprintf("ok (%d)", total),
		Data: &pb.GetTransferBatchData{
			Batch: pbBatch,
			Summary: &pb.TransferBatchDetailSummary{
				TotalRecipients: batch.TotalRecipients,
				TotalAmount:     batch.TotalAmount,
				TotalAmountUsd:  batch.TotalAmountUSD,
				TotalFee:        "0",
				TotalFeeUsd:     "0",
				SuccessCount:    successCnt,
				FailedCount:     failedCnt,
				PendingCount:    pendingCnt,
			},
			Recipients:      respItems,
			Timeline:        timeline,
			CurrencySummary: respCurrencySummary,
		},
		Pagination: p,
		RequestId:  resp.RequestID(l.ctx),
		Timestamp:  resp.Timestamp(),
	}, nil
}

type transferBatchTimelineAuditDetails struct {
	Note        string   `json:"note"`
	Reason      string   `json:"reason"`
	Action      string   `json:"action"`
	TransferIds []string `json:"transfer_ids"`
	Biz         *struct {
		Note        string   `json:"note"`
		Reason      string   `json:"reason"`
		Action      string   `json:"action"`
		TransferIds []string `json:"transfer_ids"`
	} `json:"biz"`
}

func (d *transferBatchTimelineAuditDetails) getNote() string {
	if d == nil {
		return ""
	}
	if d.Biz != nil && strings.TrimSpace(d.Biz.Note) != "" {
		return strings.TrimSpace(d.Biz.Note)
	}
	return strings.TrimSpace(d.Note)
}

func (d *transferBatchTimelineAuditDetails) getTransferIDs() []string {
	if d == nil {
		return nil
	}
	if d.Biz != nil && len(d.Biz.TransferIds) > 0 {
		return d.Biz.TransferIds
	}
	if len(d.TransferIds) > 0 {
		return d.TransferIds
	}
	return nil
}

func (d *transferBatchTimelineAuditDetails) getReason() string {
	if d == nil {
		return ""
	}
	if d.Biz != nil && strings.TrimSpace(d.Biz.Reason) != "" {
		return strings.TrimSpace(d.Biz.Reason)
	}
	return strings.TrimSpace(d.Reason)
}

func (d *transferBatchTimelineAuditDetails) getAction() string {
	if d == nil {
		return ""
	}
	if d.Biz != nil && strings.TrimSpace(d.Biz.Action) != "" {
		return strings.TrimSpace(d.Biz.Action)
	}
	return strings.TrimSpace(d.Action)
}

func formatRetryTimelineNote(note string, rawIDs []string) string {
	note = strings.TrimSpace(note)
	if note != "" {
		lower := strings.ToLower(note)
		// Avoid noisy duplicates if operator already pasted transfer_id(s) into note.
		if strings.Contains(lower, "transfer_id") || strings.Contains(lower, "transferids") {
			return note
		}
	}

	ids := make([]string, 0, len(rawIDs))
	seen := map[string]struct{}{}
	for _, id := range rawIDs {
		v := strings.TrimSpace(id)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		ids = append(ids, v)
	}

	key := "transfer_ids"
	value := "ALL_FAILED"
	if len(ids) == 1 {
		key = "transfer_id"
		value = ids[0]
	} else if len(ids) > 1 {
		const maxValueLen = 200
		var b strings.Builder
		remain := 0
		for i, id := range ids {
			part := id
			if i > 0 {
				part = "," + part
			}
			if b.Len()+len(part) > maxValueLen {
				remain = len(ids) - i
				break
			}
			b.WriteString(part)
		}
		if remain > 0 {
			if b.Len() > 0 {
				b.WriteString(fmt.Sprintf("...(+%d)", remain))
			} else {
				// Defensive: if a single id is ridiculously long.
				b.WriteString(fmt.Sprintf("...(+%d)", len(ids)))
			}
		}
		value = b.String()
	}

	suffix := fmt.Sprintf("%s=%s", key, value)
	if note == "" {
		return suffix
	}
	return note + " | " + suffix
}

func (l *GetTransferBatchLogic) getTransferBatchTimelineNotesAndExtra(batchID string) (map[string]string, []*pb.TransferBatchTimelineItem) {
	batchID = strings.TrimSpace(batchID)
	if batchID == "" || l == nil || l.svcCtx == nil || l.svcCtx.DB == nil {
		return nil, nil
	}

	actions := []string{
		"transfer.batch.submit",
		"transfer.batch.approve",
		"transfer.batch.execute",
		"transfer.batch.cancel",
		"transfer.batch.retry",
		"transfer.batch.import",
	}

	type auditRow struct {
		Action        string    `gorm:"column:action"`
		Details       []byte    `gorm:"column:details"`
		CreatedAt     time.Time `gorm:"column:created_at"`
		OperatorEmail string    `gorm:"column:operator_email"`
	}
	var rows []auditRow
	if err := l.svcCtx.DB.WithContext(l.ctx).
		Table("admin_audit_logs").
		Select("action, details, created_at, operator_email").
		Where("deleted_at IS NULL AND target_type = ? AND target_id = ? AND action IN ?", "transfer_batch", batchID, actions).
		Order("id ASC").
		Scan(&rows).Error; err != nil {
		l.Logger.Errorf("get transfer batch audit logs failed: %v", err)
		return nil, nil
	}

	noteByStatus := map[string]string{}
	extraTimeline := make([]*pb.TransferBatchTimelineItem, 0)
	for _, r := range rows {
		act := strings.TrimSpace(r.Action)
		if act == "" || len(r.Details) == 0 {
			continue
		}
		var d transferBatchTimelineAuditDetails
		if err := json.Unmarshal(r.Details, &d); err != nil {
			continue
		}

		switch act {
		case "transfer.batch.submit":
			if v := d.getNote(); v != "" {
				noteByStatus["pending"] = v
			}
		case "transfer.batch.approve":
			// action is approve|reject inside details
			subAction := strings.ToLower(strings.TrimSpace(d.getAction()))
			if subAction == "reject" {
				// 走取消逻辑时，备注会写在 note（并落库到 cancel_reason）
				if v := d.getNote(); v != "" {
					noteByStatus["cancelled"] = v
				}
			} else {
				if v := d.getNote(); v != "" {
					noteByStatus["approved"] = v
				}
			}
		case "transfer.batch.execute":
			if v := d.getNote(); v != "" {
				noteByStatus["processing"] = v
			}
		case "transfer.batch.cancel":
			// cancel 使用 reason 字段
			if v := d.getReason(); v != "" {
				noteByStatus["cancelled"] = v
			} else if v := d.getNote(); v != "" {
				noteByStatus["cancelled"] = v
			}
		case "transfer.batch.retry":
			// Show retry events in timeline (operator + note).
			if v := d.getNote(); v != "" {
				op := strings.TrimSpace(r.OperatorEmail)
				if op == "" {
					op = "system"
				}
				extraTimeline = append(extraTimeline, &pb.TransferBatchTimelineItem{
					Status:    "retry",
					Timestamp: formatTime(r.CreatedAt),
					Operator:  op,
					Note:      formatRetryTimelineNote(v, d.getTransferIDs()),
				})
			}
		case "transfer.batch.import":
			// import 目前不在时间线状态里展示；如未来需要可扩展
		}
	}

	if len(noteByStatus) == 0 {
		noteByStatus = nil
	}
	if len(extraTimeline) == 0 {
		extraTimeline = nil
	}
	return noteByStatus, extraTimeline
}

func buildTransferBatchTimeline(batch *pb.TransferBatchItem, noteByStatus map[string]string) []*pb.TransferBatchTimelineItem {
	if batch == nil {
		return nil
	}
	out := make([]*pb.TransferBatchTimelineItem, 0, 6)
	if strings.TrimSpace(batch.CreatedAt) != "" {
		draftNote := strings.TrimSpace(noteByStatus["draft"])
		if draftNote == "" {
			draftNote = strings.TrimSpace(batch.Description)
		}
		out = append(out, &pb.TransferBatchTimelineItem{
			Status:    "draft",
			Timestamp: batch.CreatedAt,
			Operator:  batch.CreatedBy,
			Note:      draftNote,
		})
	}
	if strings.TrimSpace(batch.SubmittedAt) != "" {
		out = append(out, &pb.TransferBatchTimelineItem{
			Status:    "pending",
			Timestamp: batch.SubmittedAt,
			Operator:  batch.CreatedBy,
			Note:      strings.TrimSpace(noteByStatus["pending"]),
		})
	}
	if strings.TrimSpace(batch.ApprovedAt) != "" {
		out = append(out, &pb.TransferBatchTimelineItem{
			Status:    "approved",
			Timestamp: batch.ApprovedAt,
			Operator:  batch.ApprovedBy,
			Note:      strings.TrimSpace(noteByStatus["approved"]),
		})
	}
	if strings.TrimSpace(batch.ProcessedAt) != "" {
		out = append(out, &pb.TransferBatchTimelineItem{
			Status:    "processing",
			Timestamp: batch.ProcessedAt,
			Operator:  "system",
			Note:      strings.TrimSpace(noteByStatus["processing"]),
		})
	}
	if strings.TrimSpace(batch.CompletedAt) != "" {
		out = append(out, &pb.TransferBatchTimelineItem{
			Status:    "completed",
			Timestamp: batch.CompletedAt,
			Operator:  "system",
			Note:      strings.TrimSpace(noteByStatus["completed"]),
		})
	}
	if strings.TrimSpace(batch.Status) == "cancelled" && strings.TrimSpace(batch.CancelledAt) != "" {
		op := strings.TrimSpace(batch.CancelledBy)
		if op == "" {
			op = batch.CreatedBy
		}
		out = append(out, &pb.TransferBatchTimelineItem{
			Status:    "cancelled",
			Timestamp: batch.CancelledAt,
			Operator:  op,
			Note:      strings.TrimSpace(noteByStatus["cancelled"]),
		})
	}
	return out
}

func sortTransferBatchTimeline(items []*pb.TransferBatchTimelineItem) {
	if len(items) <= 1 {
		return
	}
	parse := func(s string) time.Time {
		s = strings.TrimSpace(s)
		if s == "" {
			return time.Time{}
		}
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			return time.Time{}
		}
		return t
	}
	sort.SliceStable(items, func(i, j int) bool {
		ti := parse(items[i].Timestamp)
		tj := parse(items[j].Timestamp)
		return ti.Before(tj)
	})
}
