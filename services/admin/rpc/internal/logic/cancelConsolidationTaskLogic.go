package logic

import (
	"context"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
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

func (l *CancelConsolidationTaskLogic) CancelConsolidationTask(in *pb.AdminCancelConsolidationTaskRequest) (*pb.AdminCancelConsolidationTaskResponse, error) {
	taskID := ""
	if in != nil {
		taskID = strings.TrimSpace(in.TaskId)
	}
	if taskID == "" {
		return &pb.AdminCancelConsolidationTaskResponse{
			Success:   false,
			Message:   "task_id is required",
			RequestId: resp.RequestID(l.ctx),
			Timestamp: resp.Timestamp(),
		}, nil
	}
	if l.svcCtx == nil || l.svcCtx.ConsolidationRpc == nil {
		return &pb.AdminCancelConsolidationTaskResponse{
			Success:   false,
			Message:   "Consolidation服务未配置或未启动",
			RequestId: resp.RequestID(l.ctx),
			Timestamp: resp.Timestamp(),
		}, nil
	}

	rpcResp, err := l.svcCtx.ConsolidationRpc.CancelConsolidationTask(l.ctx, &pb.CancelConsolidationTaskRequest{TaskId: taskID})
	if err != nil {
		l.Logger.Errorf("call consolidation.CancelConsolidationTask failed: %v", err)
		return &pb.AdminCancelConsolidationTaskResponse{
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
	return &pb.AdminCancelConsolidationTaskResponse{
		Success:   rpcResp != nil && rpcResp.Success,
		Message:   msg,
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
