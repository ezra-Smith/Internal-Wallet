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

type RetryTransferBatchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRetryTransferBatchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RetryTransferBatchLogic {
	return &RetryTransferBatchLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *RetryTransferBatchLogic) RetryTransferBatch(in *pb.RetryTransferBatchRequest) (*pb.RetryTransferBatchResponse, error) {
	if in == nil || strings.TrimSpace(in.BatchId) == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "batch_id required", map[string]string{"batch_id": "required"})
	}
	// Prevent entering a stuck state if the background worker is not running.
	if !l.svcCtx.Config.TransferBatchWorker.Enabled {
		return nil, errx.New(codes.FailedPrecondition, 409, errx.CodeConflict, "TRANSFER_BATCH_WORKER_DISABLED", "transfer batch worker is disabled", map[string]string{"worker": "disabled"})
	}
	if l.svcCtx.DB == nil || l.svcCtx.TransferBatchItemRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	note := strings.TrimSpace(in.Note)
	if note == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "note required", map[string]string{"note": "required"})
	}
	if len(note) > 256 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "note too long", map[string]string{"note": "max 256"})
	}

	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}
	if err := verifyAdminTwoFA(l.svcCtx, l.ctx, current.ID, in.TwoFaCode); err != nil {
		return nil, err
	}

	now := time.Now()
	ip := middleware.GetClientIP(l.ctx)
	ua := middleware.GetUserAgent(l.ctx)

	batchID := strings.TrimSpace(in.BatchId)

	transferIDs := make([]string, 0, len(in.TransferIds))
	seenID := map[string]struct{}{}
	for _, id := range in.TransferIds {
		v := strings.TrimSpace(id)
		if v == "" {
			continue
		}
		if _, ok := seenID[v]; ok {
			continue
		}
		seenID[v] = struct{}{}
		transferIDs = append(transferIDs, v)
	}
	if len(in.TransferIds) > 0 && len(transferIDs) == 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "transfer_ids invalid", map[string]string{"transfer_ids": "invalid"})
	}

	var affected int64
	if err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		var m model.TransferBatchModel
		if err := tx.WithContext(l.ctx).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("batch_id = ?", batchID).
			First(&m).Error; err != nil {
			return err
		}
		if m.Status != "processing" && m.Status != "partial_failed" {
			return errx.New(codes.FailedPrecondition, 409, errx.CodeConflict, "BATCH_NOT_PROCESSING", "batch is not processing", map[string]string{"status": m.Status})
		}

		// Strict validation: only failed items can be retried (single/batch).
		if len(transferIDs) > 0 {
			type itemRow struct {
				ID     string `gorm:"column:id"`
				Status string `gorm:"column:status"`
			}
			var rows []itemRow
			if err := tx.WithContext(l.ctx).
				Clauses(clause.Locking{Strength: "UPDATE"}).
				Model(&model.TransferBatchItemModel{}).
				Select("id, status").
				Where("batch_id = ? AND id IN ?", batchID, transferIDs).
				Find(&rows).Error; err != nil {
				return err
			}

			rowByID := map[string]itemRow{}
			for _, r := range rows {
				id := strings.TrimSpace(r.ID)
				if id == "" {
					continue
				}
				rowByID[id] = r
			}

			invalidIDs := make([]string, 0)
			for _, id := range transferIDs {
				r, ok := rowByID[id]
				if !ok {
					invalidIDs = append(invalidIDs, id)
					continue
				}
				if strings.TrimSpace(r.Status) != "failed" {
					invalidIDs = append(invalidIDs, id)
				}
			}
			if len(invalidIDs) > 0 {
				return errx.New(codes.FailedPrecondition, 409, errx.CodeConflict, "INVALID_RETRY_ITEMS", "only failed items can be retried", map[string]string{
					"invalid_transfer_ids": strings.Join(invalidIDs, ","),
				})
			}
		}

		itemRepo := repository.NewTransferBatchItemRepository(tx)
		n, err := itemRepo.MarkRetrying(l.ctx, batchID, transferIDs)
		if err != nil {
			return err
		}
		if len(transferIDs) > 0 && n != int64(len(transferIDs)) {
			// Defensive: if strict validation passes, updates should match selected count.
			return errx.New(codes.Internal, 500, errx.CodeInternalError, "RETRY_COUNT_MISMATCH", "retry update count mismatch", nil)
		}
		if len(transferIDs) == 0 && n <= 0 {
			return errx.New(codes.FailedPrecondition, 409, errx.CodeConflict, "NO_FAILED_ITEMS", "no failed items to retry", nil)
		}
		affected = n

		// If the batch was already finished (partial_failed), move it back to processing when retries are scheduled.
		if affected > 0 && strings.TrimSpace(m.Status) == "partial_failed" {
			if err := tx.WithContext(l.ctx).
				Model(&model.TransferBatchModel{}).
				Where("batch_id = ?", batchID).
				Updates(map[string]interface{}{
					"status":       "processing",
					"processed_at": &now,
					"completed_at": nil,
				}).Error; err != nil {
				return err
			}
		}

		detailsBytes, _ := json.Marshal(map[string]interface{}{
			"batch_id":     batchID,
			"transfer_ids": transferIDs,
			"retry_count":  affected,
			"note":         note,
			"requested_at": formatTime(now),
			"batch_status": m.Status,
		})
		auditRepo := repository.NewAdminAuditLogRepository(tx)
		_ = auditRepo.CreateLog(l.ctx, &model.AdminAuditLogModel{
			AdminID:     current.ID,
			Action:      "transfer.batch.retry",
			TargetType:  "transfer_batch",
			TargetID:    batchID,
			Description: "重试转账批次失败明细",
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
		l.Logger.Errorf("retry transfer batch failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	return &pb.RetryTransferBatchResponse{
		Success: true,
		Message: resp.Msg(l.ctx, "RETRY_TASK_STARTED"),
		Data: &pb.RetryTransferBatchData{
			BatchId:    strings.TrimSpace(in.BatchId),
			RetryCount: int32(affected),
			Status:     "processing",
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
