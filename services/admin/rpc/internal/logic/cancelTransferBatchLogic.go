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

type CancelTransferBatchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCancelTransferBatchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CancelTransferBatchLogic {
	return &CancelTransferBatchLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CancelTransferBatchLogic) CancelTransferBatch(in *pb.CancelTransferBatchRequest) (*pb.CancelTransferBatchResponse, error) {
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

	now := time.Now()
	ip := middleware.GetClientIP(l.ctx)
	ua := middleware.GetUserAgent(l.ctx)
	reason := strings.TrimSpace(in.Reason)

	if err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		var m model.TransferBatchModel
		if err := tx.WithContext(l.ctx).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("batch_id = ?", strings.TrimSpace(in.BatchId)).
			First(&m).Error; err != nil {
			return err
		}
		if m.Status != "draft" && m.Status != "pending" && m.Status != "approved" {
			return errx.New(codes.FailedPrecondition, 409, errx.CodeConflict, "BATCH_CANNOT_CANCEL", "cannot cancel batch", map[string]string{"status": m.Status})
		}

		// 将批次内所有未完成的转账项标记为失败
		cancelReason := reason
		if cancelReason == "" {
			cancelReason = "批次已取消"
		}
		if err := tx.WithContext(l.ctx).
			Model(&model.TransferBatchItemModel{}).
			Where("batch_id = ? AND status IN ?", strings.TrimSpace(in.BatchId), []string{"pending", "retrying", "processing"}).
			Updates(map[string]interface{}{
				"status":        "failed",
				"processed_at":  &now,
				"error_message": cancelReason,
			}).Error; err != nil {
			l.Logger.Errorf("failed to update batch items status: %v", err)
			return err
		}

		if err := tx.WithContext(l.ctx).
			Model(&model.TransferBatchModel{}).
			Where("batch_id = ?", strings.TrimSpace(in.BatchId)).
			Updates(map[string]interface{}{
				"status":          "cancelled",
				"cancelled_at":    &now,
				"cancelled_by":    current.Username,
				"cancelled_by_id": current.ID,
				"cancel_reason":   strPtrOrNilTrim(reason),
			}).Error; err != nil {
			return err
		}

		detailsBytes, _ := json.Marshal(map[string]interface{}{
			"batch_id":    in.BatchId,
			"reason":      reason,
			"status_from": m.Status,
			"status_to":   "cancelled",
		})
		auditRepo := repository.NewAdminAuditLogRepository(tx)
		_ = auditRepo.CreateLog(l.ctx, &model.AdminAuditLogModel{
			AdminID:     current.ID,
			Action:      "transfer.batch.cancel",
			TargetType:  "transfer_batch",
			TargetID:    in.BatchId,
			Description: "取消转账批次",
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
		l.Logger.Errorf("cancel transfer batch failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	return &pb.CancelTransferBatchResponse{
		Success: true,
		Message: resp.Msg(l.ctx, "BATCH_CANCELLED"),
		Data: &pb.CancelTransferBatchData{
			BatchId:     strings.TrimSpace(in.BatchId),
			Status:      "cancelled",
			CancelledAt: formatTime(now),
			CancelledBy: current.Username,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
