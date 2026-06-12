package logic

import (
	"context"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
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

func (l *RetryConsolidationTaskLogic) RetryConsolidationTask(in *pb.AdminRetryConsolidationTaskRequest) (*pb.AdminRetryConsolidationTaskResponse, error) {
	taskID := ""
	if in != nil {
		taskID = strings.TrimSpace(in.TaskId)
	}
	if taskID == "" {
		return &pb.AdminRetryConsolidationTaskResponse{
			Success:   false,
			Message:   "task_id is required",
			RequestId: resp.RequestID(l.ctx),
			Timestamp: resp.Timestamp(),
		}, nil
	}
	if l.svcCtx == nil || l.svcCtx.ConsolidationRpc == nil {
		return &pb.AdminRetryConsolidationTaskResponse{
			Success:   false,
			Message:   "Consolidation服务未配置或未启动",
			RequestId: resp.RequestID(l.ctx),
			Timestamp: resp.Timestamp(),
		}, nil
	}

	rpcResp, err := l.svcCtx.ConsolidationRpc.RetryConsolidationTask(l.ctx, &pb.RetryConsolidationTaskRequest{TaskId: taskID})
	if err != nil {
		l.Logger.Errorf("call consolidation.RetryConsolidationTask failed: %v", err)
		return &pb.AdminRetryConsolidationTaskResponse{
			Success:   false,
			Message:   "调用Consolidation服务失败",
			RequestId: resp.RequestID(l.ctx),
			Timestamp: resp.Timestamp(),
		}, nil
	}

	msg := "ok"
	if rpcResp != nil && rpcResp.Message != "" {
		msg = rpcResp.Message
	}
	return &pb.AdminRetryConsolidationTaskResponse{
		Success:   rpcResp != nil && rpcResp.Success,
		Message:   msg,
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
