package logic

import (
	"context"
	"strconv"
	"strings"
	"time"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/model"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetWithdrawAuditStatusLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetWithdrawAuditStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetWithdrawAuditStatusLogic {
	return &GetWithdrawAuditStatusLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetWithdrawAuditStatusLogic) GetWithdrawAuditStatus(in *pb.GetWithdrawAuditStatusReq) (*pb.GetWithdrawAuditStatusResp, error) {
	if in == nil || in.WithdrawId == "" {
		return nil, errx.InvalidParam("invalid params")
	}
	uidStr := middleware.GetUserID(l.ctx)
	if uidStr == "" {
		return nil, errx.Unauthorized("unauthorized")
	}
	userID, _ := strconv.ParseInt(uidStr, 10, 64)
	withdrawID, err := strconv.ParseInt(strings.TrimSpace(in.WithdrawId), 10, 64)
	if userID <= 0 || err != nil || withdrawID <= 0 {
		return nil, errx.InvalidParam("invalid withdraw_id")
	}

	// Prefer currency_withdraw_orders when available.
	if l.svcCtx.CurrencyWithdrawOrderRepository != nil {
		if order, err := l.svcCtx.CurrencyWithdrawOrderRepository.FindByUserIDAndID(l.ctx, userID, withdrawID); err == nil && order != nil {
			return l.buildWithdrawAuditStatusFromOrder(order), nil
		}
	}

	return nil, errx.WithdrawNotFound()
}

func (l *GetWithdrawAuditStatusLogic) buildWithdrawAuditStatusFromOrder(order *model.CurrencyWithdrawOrderModel) *pb.GetWithdrawAuditStatusResp {
	created := order.CreatedAt
	updated := order.UpdatedAt
	strategy := strings.ToLower(strings.TrimSpace(order.Strategy))
	status := strings.TrimSpace(order.Status)
	if status == "" {
		status = "pending"
	}

	auditType := "manual"
	if strategy == "auto" {
		auditType = "auto"
	}

	var auditStatus string
	var auditStatusText string
	switch strategy {
	case "auto":
		switch status {
		case "failed":
			auditStatus = "failed"
			auditStatusText = "失败"
		case "cancelled":
			auditStatus = "cancelled"
			auditStatusText = "已取消"
		case "completed":
			auditStatus = "completed"
			auditStatusText = "已完成"
		default:
			auditStatus = "processing"
			auditStatusText = "处理中"
		}
	default:
		// manual_* strategies
		if status == "cancelled" {
			auditStatus = "cancelled"
			auditStatusText = "已取消"
		} else if status == "failed" && order.AuditedAt != nil {
			auditStatus = "rejected"
			auditStatusText = "已拒绝"
		} else if order.AuditedAt != nil {
			auditStatus = "approved"
			if strategy == "manual_manual" && order.TransferredAt == nil {
				auditStatusText = "已通过（待放币）"
			} else {
				auditStatusText = "已通过"
			}
		} else {
			auditStatus = "reviewing"
			auditStatusText = "审核中"
		}
	}

	reason := ""
	if order.ErrorMessage != nil && strings.TrimSpace(*order.ErrorMessage) != "" {
		reason = strings.TrimSpace(*order.ErrorMessage)
	} else if order.AuditNote != nil && strings.TrimSpace(*order.AuditNote) != "" {
		reason = strings.TrimSpace(*order.AuditNote)
	}

	timeline := make([]*pb.AuditTimelineItem, 0, 6)
	timeline = append(timeline, &pb.AuditTimelineItem{
		Status:      "submitted",
		Time:        created.Format(time.RFC3339),
		Description: "提交申请",
	})
	if strategy != "auto" {
		timeline = append(timeline, &pb.AuditTimelineItem{
			Status:      "reviewing",
			Time:        created.Format(time.RFC3339),
			Description: "人工审核",
		})
		if order.AuditedAt != nil {
			desc := "审核通过"
			if status == "failed" {
				desc = "审核拒绝"
				if reason != "" {
					desc += "：" + reason
				}
			}
			timeline = append(timeline, &pb.AuditTimelineItem{
				Status:      "audit_done",
				Time:        order.AuditedAt.Format(time.RFC3339),
				Description: desc,
			})
		}
	}
	if order.TransferredAt != nil {
		timeline = append(timeline, &pb.AuditTimelineItem{
			Status:      "payout",
			Time:        order.TransferredAt.Format(time.RFC3339),
			Description: "放币处理",
		})
	}
	switch status {
	case "completed":
		timeline = append(timeline, &pb.AuditTimelineItem{
			Status:      "completed",
			Time:        updated.Format(time.RFC3339),
			Description: "已完成",
		})
	case "failed":
		desc := "失败"
		if reason != "" {
			desc += "：" + reason
		}
		timeline = append(timeline, &pb.AuditTimelineItem{
			Status:      "failed",
			Time:        updated.Format(time.RFC3339),
			Description: desc,
		})
	case "cancelled":
		timeline = append(timeline, &pb.AuditTimelineItem{
			Status:      "cancelled",
			Time:        updated.Format(time.RFC3339),
			Description: "已取消",
		})
	}

	return &pb.GetWithdrawAuditStatusResp{
		Success:            true,
		WithdrawId:         strconv.FormatInt(order.ID, 10),
		Status:             auditStatus,
		StatusText:         auditStatusText,
		SubmittedAt:        created.Format(time.RFC3339),
		AuditStartedAt:     created.Format(time.RFC3339),
		AuditType:          auditType,
		AuditReason:        reason,
		ExpectedCompletion: "",
		Timeline:           timeline,
	}
}
