package logic

import (
	"context"
	"errors"

	"internalwallet/proto/pb"
	"internalwallet/services/consolidation/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type GetConsolidationTaskLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetConsolidationTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetConsolidationTaskLogic {
	return &GetConsolidationTaskLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Query task by task_id.
func (l *GetConsolidationTaskLogic) GetConsolidationTask(in *pb.GetConsolidationTaskRequest) (*pb.GetConsolidationTaskResponse, error) {
	if in == nil || in.TaskId == "" {
		return &pb.GetConsolidationTaskResponse{Success: false, Message: "task_id is required"}, nil
	}
	if l.svcCtx == nil || l.svcCtx.ConsolidationTaskRepo == nil {
		return &pb.GetConsolidationTaskResponse{Success: false, Message: "service not ready"}, nil
	}
	task, err := l.svcCtx.ConsolidationTaskRepo.GetByTaskID(l.ctx, in.TaskId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &pb.GetConsolidationTaskResponse{Success: false, Message: "not found"}, nil
		}
		return &pb.GetConsolidationTaskResponse{Success: false, Message: err.Error()}, nil
	}
	return &pb.GetConsolidationTaskResponse{
		Success: true,
		Message: "ok",
		Task:    toProtoTask(task),
	}, nil
}
