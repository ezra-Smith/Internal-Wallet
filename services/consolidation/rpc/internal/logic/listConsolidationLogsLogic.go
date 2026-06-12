package logic

import (
	"context"
	"strings"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/consolidation/rpc/internal/repository"
	"internalwallet/services/consolidation/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListConsolidationLogsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListConsolidationLogsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListConsolidationLogsLogic {
	return &ListConsolidationLogsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListConsolidationLogsLogic) ListConsolidationLogs(in *pb.ListConsolidationLogsRequest) (*pb.ListConsolidationLogsResponse, error) {
	if in == nil {
		return &pb.ListConsolidationLogsResponse{Success: false, Message: "request is required"}, nil
	}
	if l.svcCtx == nil || l.svcCtx.ConsolidationLogRepo == nil {
		return &pb.ListConsolidationLogsResponse{Success: false, Message: "service not ready"}, nil
	}

	var taskID *string
	if strings.TrimSpace(in.TaskId) != "" {
		v := strings.TrimSpace(in.TaskId)
		taskID = &v
	}
	var level *string
	if strings.TrimSpace(in.LogLevel) != "" {
		v := strings.TrimSpace(in.LogLevel)
		level = &v
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

	logs, total, err := l.svcCtx.ConsolidationLogRepo.List(l.ctx, repository.ListLogsRequest{
		Page:          in.Page,
		PageSize:      in.PageSize,
		TaskID:        taskID,
		LogLevel:      level,
		CreatedAtFrom: from,
		CreatedAtTo:   to,
	})
	if err != nil {
		return &pb.ListConsolidationLogsResponse{Success: false, Message: err.Error()}, nil
	}

	resp := &pb.ListConsolidationLogsResponse{
		Success: true,
		Message: "ok",
		Total:   total,
		Logs:    make([]*pb.ConsolidationLog, 0, len(logs)),
	}
	for i := range logs {
		item := logs[i]
		resp.Logs = append(resp.Logs, toProtoLog(&item))
	}
	return resp, nil
}
