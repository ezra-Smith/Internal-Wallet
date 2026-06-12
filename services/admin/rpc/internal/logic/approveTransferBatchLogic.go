package logic

import (
	"context"
	"encoding/json"
	"errors"
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
	"gorm.io/gorm/clause"
)

type ApproveTransferBatchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewApproveTransferBatchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApproveTransferBatchLogic {
	return &ApproveTransferBatchLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ApproveTransferBatchLogic) ApproveTransferBatch(in *pb.ApproveTransferBatchRequest) (*pb.ApproveTransferBatchResponse, error) {
	if in == nil || strings.TrimSpace(in.BatchId) == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "batch_id required", map[string]string{"batch_id": "required"})
	}
	if l.svcCtx.DB == nil || l.svcCtx.TransferBatchRepo == nil {
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
	if err := verifyAdminTwoFA(l.svcCtx, l.ctx, current.ID, in.TwoFaCode); err != nil {
		return nil, err
	}

	now := time.Now()
	ip := middleware.GetClientIP(l.ctx)
	ua := middleware.GetUserAgent(l.ctx)
	note := strings.TrimSpace(in.Note)

	var statusTo string
	if action == "approve" {
		statusTo = "approved"
	} else {
		statusTo = "cancelled"
	}

	if err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		var m model.TransferBatchModel
		if err := tx.WithContext(l.ctx).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("batch_id = ?", strings.TrimSpace(in.BatchId)).
			First(&m).Error; err != nil {
			return err
		}
		if m.Status != "pending" {
			return errx.New(codes.FailedPrecondition, 409, errx.CodeConflict, "BATCH_NOT_PENDING", "batch is not pending", map[string]string{"status": m.Status})
		}
		// Self-approval check disabled - users can now approve their own batches
		// if m.CreatedByID > 0 && m.CreatedByID == current.ID {
		// 	return errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "CANNOT_APPROVE_OWN", "cannot approve own batch", nil)
		// }

		updates := map[string]interface{}{
			"status": statusTo,
		}
		if action == "approve" {
			updates["approved_by"] = current.Username
			updates["approved_by_id"] = current.ID
			updates["approved_at"] = &now
		} else {
			updates["cancelled_by"] = current.Username
			updates["cancelled_by_id"] = current.ID
			updates["cancelled_at"] = &now
			updates["cancel_reason"] = strPtrOrNilTrim(note)
		}

		if err := tx.WithContext(l.ctx).
			Model(&model.TransferBatchModel{}).
			Where("batch_id = ?", strings.TrimSpace(in.BatchId)).
			Updates(updates).Error; err != nil {
			return err
		}

		detailsBytes, _ := json.Marshal(map[string]interface{}{
			"batch_id":    in.BatchId,
			"action":      action,
			"note":        note,
			"status_from": "pending",
			"status_to":   statusTo,
		})
		auditRepo := repository.NewAdminAuditLogRepository(tx)
		_ = auditRepo.CreateLog(l.ctx, &model.AdminAuditLogModel{
			AdminID:     current.ID,
			Action:      "transfer.batch.approve",
			TargetType:  "transfer_batch",
			TargetID:    in.BatchId,
			Description: "审批转账批次",
			Details:     detailsBytes,
			IP:          ip,
			UserAgent:   ua,
		})
		return nil
	}); err != nil {
		if _, ok := status.FromError(err); ok {
			return nil, err
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "BATCH_NOT_FOUND", "batch not found", nil)
		}
		l.Logger.Errorf("approve transfer batch failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	data := &pb.ApproveTransferBatchData{
		BatchId:    strings.TrimSpace(in.BatchId),
		Status:     statusTo,
		ApprovedAt: "",
		ApprovedBy: "",
	}
	if action == "approve" {
		data.ApprovedAt = formatTime(now)
		data.ApprovedBy = current.Username
	}
	return &pb.ApproveTransferBatchResponse{
		Success:   true,
		Message:   "ok",
		Data:      data,
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
