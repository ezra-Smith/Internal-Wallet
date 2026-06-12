package logic

import (
	"context"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/consolidation/rpc/internal/models"
	"internalwallet/services/consolidation/rpc/internal/repository"
	"internalwallet/services/consolidation/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListConsolidationTasksLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListConsolidationTasksLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListConsolidationTasksLogic {
	return &ListConsolidationTasksLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// List tasks with filters.
func (l *ListConsolidationTasksLogic) ListConsolidationTasks(in *pb.ListConsolidationTasksRequest) (*pb.ListConsolidationTasksResponse, error) {
	if in == nil {
		return &pb.ListConsolidationTasksResponse{Success: false, Message: "request is required"}, nil
	}
	if l.svcCtx == nil || l.svcCtx.ConsolidationTaskRepo == nil {
		return &pb.ListConsolidationTasksResponse{Success: false, Message: "service not ready"}, nil
	}

	var chain *string
	if in.Chain != "" {
		v := in.Chain
		chain = &v
	}
	var asset *string
	if in.AssetSymbol != "" {
		v := in.AssetSymbol
		asset = &v
	}

	var taskID *string
	if in.TaskId != "" {
		v := in.TaskId
		taskID = &v
	}
	var fromAddress *string
	if in.FromAddress != "" {
		v := in.FromAddress
		fromAddress = &v
	}
	var toAddress *string
	if in.ToAddress != "" {
		v := in.ToAddress
		toAddress = &v
	}
	var txHash *string
	if in.TxHash != "" {
		v := in.TxHash
		txHash = &v
	}

	var status *models.ConsolidationTaskStatus
	if in.Status != pb.ConsolidationTaskStatus_CONSOLIDATION_TASK_STATUS_UNSPECIFIED && in.Status != 0 {
		var s models.ConsolidationTaskStatus
		switch in.Status {
		case pb.ConsolidationTaskStatus_CONSOLIDATION_TASK_STATUS_PENDING:
			s = models.ConsolidationTaskStatusPending
		case pb.ConsolidationTaskStatus_CONSOLIDATION_TASK_STATUS_NEED_ENERGY:
			s = models.ConsolidationTaskStatusNeedEnergy
		case pb.ConsolidationTaskStatus_CONSOLIDATION_TASK_STATUS_IN_PROGRESS:
			s = models.ConsolidationTaskStatusInProgress
		case pb.ConsolidationTaskStatus_CONSOLIDATION_TASK_STATUS_CONFIRMED:
			s = models.ConsolidationTaskStatusConfirmed
		case pb.ConsolidationTaskStatus_CONSOLIDATION_TASK_STATUS_FAILED:
			s = models.ConsolidationTaskStatusFailed
		case pb.ConsolidationTaskStatus_CONSOLIDATION_TASK_STATUS_TIMEOUT:
			s = models.ConsolidationTaskStatusTimeout
		case pb.ConsolidationTaskStatus_CONSOLIDATION_TASK_STATUS_NEED_GAS:
			s = models.ConsolidationTaskStatusNeedGas
		case pb.ConsolidationTaskStatus_CONSOLIDATION_TASK_STATUS_NEED_BANDWIDTH:
			s = models.ConsolidationTaskStatusNeedBandwidth
		case pb.ConsolidationTaskStatus_CONSOLIDATION_TASK_STATUS_CANCELLED:
			s = models.ConsolidationTaskStatusCancelled
		case pb.ConsolidationTaskStatus_CONSOLIDATION_TASK_STATUS_PERMANENT_FAILED:
			s = models.ConsolidationTaskStatusPermanentFailed
		default:
			s = 0
		}
		if s != 0 {
			status = &s
		}
	}

	var from *time.Time
	if in.CreatedAtFrom > 0 {
		v := time.Unix(in.CreatedAtFrom, 0).Local()
		from = &v
	}
	var to *time.Time
	if in.CreatedAtTo > 0 {
		v := time.Unix(in.CreatedAtTo, 0).Local()
		to = &v
	}

	tasks, total, err := l.svcCtx.ConsolidationTaskRepo.List(l.ctx, repository.ListTasksRequest{
		Page:          in.Page,
		PageSize:      in.PageSize,
		Chain:         chain,
		Asset:         asset,
		Status:        status,
		CreatedAtFrom: from,
		CreatedAtTo:   to,
		TaskID:        taskID,
		FromAddress:   fromAddress,
		ToAddress:     toAddress,
		TxHash:        txHash,
	})
	if err != nil {
		return &pb.ListConsolidationTasksResponse{Success: false, Message: err.Error()}, nil
	}
	resp := &pb.ListConsolidationTasksResponse{
		Success: true,
		Message: "ok",
		Total:   total,
	}
	resp.Tasks = make([]*pb.ConsolidationTask, 0, len(tasks))
	for i := range tasks {
		task := tasks[i]
		resp.Tasks = append(resp.Tasks, toProtoTask(&task))
	}
	return resp, nil
}
