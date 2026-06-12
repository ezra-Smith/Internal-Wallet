package logic

import (
	"context"
	"errors"

	"internalwallet/proto/pb"
	"internalwallet/services/consolidation/rpc/internal/models"
	"internalwallet/services/consolidation/rpc/internal/scheduler"
	"internalwallet/services/consolidation/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type CancelConsolidationTaskLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCancelConsolidationTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CancelConsolidationTaskLogic {
	return &CancelConsolidationTaskLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Cancel a pending task.
func (l *CancelConsolidationTaskLogic) CancelConsolidationTask(in *pb.CancelConsolidationTaskRequest) (*pb.CancelConsolidationTaskResponse, error) {
	if in == nil || in.TaskId == "" {
		return &pb.CancelConsolidationTaskResponse{Success: false, Message: "task_id is required"}, nil
	}
	if l.svcCtx == nil || l.svcCtx.ConsolidationTaskRepo == nil {
		return &pb.CancelConsolidationTaskResponse{Success: false, Message: "service not ready"}, nil
	}
	l.Infow("cancel consolidation task requested",
		logx.Field("event", "manual_cancel_requested"),
		logx.Field("task_id", in.TaskId),
	)
	task, err := l.svcCtx.ConsolidationTaskRepo.GetByTaskID(l.ctx, in.TaskId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &pb.CancelConsolidationTaskResponse{Success: false, Message: "not found"}, nil
		}
		l.Errorw("cancel consolidation task load failed",
			logx.Field("event", "manual_cancel_failed"),
			logx.Field("task_id", in.TaskId),
			logx.Field("error", err),
		)
		return &pb.CancelConsolidationTaskResponse{Success: false, Message: err.Error()}, nil
	}
	ok, err := l.svcCtx.ConsolidationTaskRepo.MarkCancelled(l.ctx, task.ID)
	if err != nil {
		l.Errorw("cancel consolidation task update failed",
			logx.Field("event", "manual_cancel_failed"),
			logx.Field("task_id", in.TaskId),
			logx.Field("chain", task.Chain),
			logx.Field("status", task.Status),
			logx.Field("error", err),
		)
		return &pb.CancelConsolidationTaskResponse{Success: false, Message: err.Error()}, nil
	}
	if !ok {
		l.Infow("cancel consolidation task rejected by status",
			logx.Field("event", "manual_cancel_rejected"),
			logx.Field("task_id", in.TaskId),
			logx.Field("chain", task.Chain),
			logx.Field("status", task.Status),
		)
		return &pb.CancelConsolidationTaskResponse{Success: false, Message: "task is not cancelable in current status"}, nil
	}
	scheduler.WriteTaskLog(l.ctx, l.svcCtx, task.TaskID, "INFO", scheduler.TaskLogEventTaskCancelled, "task cancelled manually", map[string]any{
		"chain":        task.Chain,
		"asset_symbol": task.AssetSymbol,
		"from_address": task.FromAddress,
		"to_address":   task.ToAddress,
		"status_from":  task.Status,
		"status_to":    models.ConsolidationTaskStatusCancelled,
	})
	l.Infow("cancel consolidation task done",
		logx.Field("event", "manual_cancel_done"),
		logx.Field("task_id", in.TaskId),
		logx.Field("chain", task.Chain),
		logx.Field("status_from", task.Status),
		logx.Field("status_to", "CANCELLED"),
	)
	return &pb.CancelConsolidationTaskResponse{Success: true, Message: "ok"}, nil
}
