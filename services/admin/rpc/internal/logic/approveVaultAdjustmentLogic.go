package logic

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	admininterceptor "internalwallet/services/admin/rpc/internal/interceptor"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/repository"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

type ApproveVaultAdjustmentLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewApproveVaultAdjustmentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApproveVaultAdjustmentLogic {
	return &ApproveVaultAdjustmentLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ApproveVaultAdjustmentLogic) ApproveVaultAdjustment(in *pb.ApproveVaultAdjustmentRequest) (*pb.ApproveVaultAdjustmentResponse, error) {
	if in == nil || in.Id <= 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid id", map[string]string{"id": "required"})
	}
	if l.svcCtx.DB == nil ||
		l.svcCtx.VaultAdjustmentRepo == nil ||
		l.svcCtx.VaultAdjustmentApprovalRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	action := strings.TrimSpace(in.Action)
	if action != "approve" && action != "reject" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_ACTION", "invalid action", map[string]string{"action": "invalid"})
	}

	note := strings.TrimSpace(in.Note)
	if len(note) > 2000 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_NOTE", "note too long", map[string]string{"note": "too long"})
	}

	if err := verifyAdminTwoFA(l.svcCtx, l.ctx, current.ID, in.TwoFaCode); err != nil {
		return nil, err
	}

	now := time.Now()
	ip := middleware.GetClientIP(l.ctx)
	ua := middleware.GetUserAgent(l.ctx)

	var updated *model.VaultAdjustmentModel
	nextStep := ""

	if err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		adjRepo := l.svcCtx.VaultAdjustmentRepo.WithTx(tx)
		approvalRepo := l.svcCtx.VaultAdjustmentApprovalRepo.WithTx(tx)

		adj, err := adjRepo.FindByIDForUpdate(l.ctx, in.Id)
		if err != nil {
			return err
		}

		if adj.Status != vaultAdjustmentStatusPending {
			return errx.New(codes.FailedPrecondition, 409, errx.CodeConflict, "ADJUSTMENT_NOT_PENDING", "adjustment not pending", map[string]string{"status": adj.Status})
		}
		if adj.SubmittedBy == current.ID {
			return errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "CANNOT_APPROVE_OWN", "cannot approve own request", nil)
		}

		exists, err := approvalRepo.ExistsByAdjustmentAndAdmin(l.ctx, adj.ID, current.ID)
		if err != nil {
			return err
		}
		if exists {
			return errx.New(codes.AlreadyExists, 409, errx.CodeConflict, "DUPLICATE_REVIEW", "duplicate review", nil)
		}

		notePtr := strPtrOrNilTrim(note)
		if err := approvalRepo.Create(l.ctx, &model.VaultAdjustmentApprovalModel{
			AdjustmentID: adj.ID,
			AdminID:      current.ID,
			Action:       action,
			Note:         notePtr,
			CreatedAt:    &now,
		}); err != nil {
			return err
		}

		fields := map[string]interface{}{
			"reviewed_by": current.ID,
			"reviewed_at": &now,
			"review_note": notePtr,
		}

		if action == "reject" {
			fields["status"] = vaultAdjustmentStatusRejected
			nextStep = ""
		} else {
			// Maker-checker:
			// - super_admin can finalize with single approval.
			// - otherwise require 2 distinct approvals.
			finalize := strings.TrimSpace(current.Role) == "super_admin"
			if !finalize {
				cnt, err := approvalRepo.CountByAdjustmentAndAction(l.ctx, adj.ID, "approve")
				if err != nil {
					return err
				}
				finalize = cnt >= 2
			}

			if finalize {
				fields["status"] = vaultAdjustmentStatusApproved
				nextStep = "等待链上确认"
			} else {
				nextStep = "等待二次复核"
			}
		}

		if err := adjRepo.UpdateFields(l.ctx, adj.ID, fields); err != nil {
			return err
		}

		out, err := adjRepo.FindByID(l.ctx, adj.ID)
		if err != nil {
			return err
		}
		updated = out

		auditRepo := repository.NewAdminAuditLogRepository(tx)
		detailsBytes, _ := json.Marshal(map[string]interface{}{
			"id":          adj.ID,
			"chain_id":    adj.ChainID,
			"network":     adj.Network,
			"currency":    adj.Currency,
			"type":        adj.AdjustmentType,
			"amount":      adj.Amount,
			"status_from": adj.Status,
			"status_to":   updated.Status,
			"action":      action,
			"note":        note,
		})
		_ = auditRepo.CreateLog(l.ctx, &model.AdminAuditLogModel{
			AdminID:     current.ID,
			Action:      "vault.adjustment.review",
			TargetType:  "vault_adjustment",
			TargetID:    strconv.FormatInt(adj.ID, 10),
			Description: "审批 Vault 调整申请",
			Details:     detailsBytes,
			IP:          ip,
			UserAgent:   ua,
		})
		return nil
	}); err != nil {
		if _, ok := status.FromError(err); ok {
			return nil, err
		}
		if strings.Contains(strings.ToLower(err.Error()), "record not found") || strings.Contains(strings.ToLower(err.Error()), "not found") {
			return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "ADJUSTMENT_NOT_FOUND", "adjustment not found", nil)
		}
		l.Logger.Errorf("approve vault adjustment failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "approve adjustment failed", nil)
	}

	msg := "ok"
	if strings.TrimSpace(updated.Status) == vaultAdjustmentStatusRejected {
		msg = "调整申请已拒绝"
	} else if strings.TrimSpace(updated.Status) == vaultAdjustmentStatusApproved {
		msg = "调整申请已审批通过"
	} else {
		msg = "已记录审批，等待复核"
	}

	return &pb.ApproveVaultAdjustmentResponse{
		Success: true,
		Message: msg,
		Data: &pb.ApproveVaultAdjustmentData{
			Adjustment: toPBVaultAdjustmentItem(updated),
			NextStep:   nextStep,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
