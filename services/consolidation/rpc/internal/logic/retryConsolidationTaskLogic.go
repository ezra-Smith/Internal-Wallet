package logic

import (
	"context"
	"errors"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/consolidation/rpc/internal/models"
	"internalwallet/services/consolidation/rpc/internal/scheduler"
	"internalwallet/services/consolidation/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type RetryConsolidationTaskLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRetryConsolidationTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RetryConsolidationTaskLogic {
	return &RetryConsolidationTaskLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Retry a failed/permanent-failed task (subject to max retry policy).
func (l *RetryConsolidationTaskLogic) RetryConsolidationTask(in *pb.RetryConsolidationTaskRequest) (*pb.RetryConsolidationTaskResponse, error) {
	if in == nil || in.TaskId == "" {
		return &pb.RetryConsolidationTaskResponse{Success: false, Message: "task_id is required"}, nil
	}
	if l.svcCtx == nil || l.svcCtx.ConsolidationTaskRepo == nil {
		return &pb.RetryConsolidationTaskResponse{Success: false, Message: "service not ready"}, nil
	}
	l.Infow("manual retry requested",
		logx.Field("event", "manual_retry_requested"),
		logx.Field("task_id", in.TaskId),
	)

	task, err := l.svcCtx.ConsolidationTaskRepo.GetByTaskID(l.ctx, in.TaskId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &pb.RetryConsolidationTaskResponse{Success: false, Message: "not found"}, nil
		}
		l.Errorw("manual retry load failed",
			logx.Field("event", "manual_retry_failed"),
			logx.Field("task_id", in.TaskId),
			logx.Field("error", err),
		)
		return &pb.RetryConsolidationTaskResponse{Success: false, Message: err.Error()}, nil
	}
	if task.Status == models.ConsolidationTaskStatusInProgress || task.Status == models.ConsolidationTaskStatusConfirmed {
		l.Infow("manual retry rejected by status",
			logx.Field("event", "manual_retry_rejected"),
			logx.Field("task_id", in.TaskId),
			logx.Field("chain", task.Chain),
			logx.Field("status", task.Status),
			logx.Field("retry_count", task.RetryCount),
		)
		return &pb.RetryConsolidationTaskResponse{Success: false, Message: "task is not retryable in current status"}, nil
	}

	// Manual retry is an operator override: reset retry_count so the task can be attempted again
	// even if it has already hit MaxRetries via automatic retries.
	nextRetryCount := int32(0)

	// Manual retry should also reset per-task top-up attempts; otherwise tasks can get stuck in
	// PermanentFailed due to TopUp.MaxTopUpAttemptsPerTask even after retry_count is reset.
	var clearedTopups int64
	if l.svcCtx.TopUpRecordRepo != nil {
		if n, err := l.svcCtx.TopUpRecordRepo.SoftDeleteByTaskID(l.ctx, task.TaskID); err != nil {
			l.Errorw("manual retry top-up history cleanup failed",
				logx.Field("event", "manual_retry_failed"),
				logx.Field("task_id", in.TaskId),
				logx.Field("error", err),
			)
			return &pb.RetryConsolidationTaskResponse{Success: false, Message: err.Error()}, nil
		} else {
			clearedTopups = n
		}
	}

	now := time.Now().Local()
	msg := "manual retry"
	ok, err := l.svcCtx.ConsolidationTaskRepo.ForceRetryPending(l.ctx, task.ID, msg, &now, nextRetryCount)
	if err != nil {
		l.Errorw("manual retry update failed",
			logx.Field("event", "manual_retry_failed"),
			logx.Field("task_id", in.TaskId),
			logx.Field("chain", task.Chain),
			logx.Field("status", task.Status),
			logx.Field("retry_count_from", task.RetryCount),
			logx.Field("retry_count_to", nextRetryCount),
			logx.Field("error", err),
		)
		return &pb.RetryConsolidationTaskResponse{Success: false, Message: err.Error()}, nil
	}
	if !ok {
		l.Infow("manual retry rejected (not retryable status)",
			logx.Field("event", "manual_retry_rejected"),
			logx.Field("task_id", in.TaskId),
			logx.Field("chain", task.Chain),
			logx.Field("status", task.Status),
		)
		return &pb.RetryConsolidationTaskResponse{Success: false, Message: "task is not retryable in current status"}, nil
	}
	scheduler.WriteTaskLog(l.ctx, l.svcCtx, task.TaskID, "INFO", scheduler.TaskLogEventTaskManualRetry, msg, map[string]any{
		"chain":            task.Chain,
		"asset_symbol":     task.AssetSymbol,
		"from_address":     task.FromAddress,
		"to_address":       task.ToAddress,
		"status_from":      task.Status,
		"status_to":        models.ConsolidationTaskStatusPending,
		"retry_count_from": task.RetryCount,
		"retry_count_to":   nextRetryCount,
		"cleared_topups":   clearedTopups,
		"next_attempt_at":  now.Format(time.RFC3339),
	})
	l.Infow("manual retry done",
		logx.Field("event", "manual_retry_done"),
		logx.Field("task_id", in.TaskId),
		logx.Field("chain", task.Chain),
		logx.Field("status_from", task.Status),
		logx.Field("status_to", models.ConsolidationTaskStatusPending),
		logx.Field("retry_count_from", task.RetryCount),
		logx.Field("retry_count_to", nextRetryCount),
	)
	return &pb.RetryConsolidationTaskResponse{Success: true, Message: "ok"}, nil
}
