package logic

import (
	"context"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
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

func (l *GetConsolidationTaskLogic) GetConsolidationTask(in *pb.AdminGetConsolidationTaskRequest) (*pb.AdminGetConsolidationTaskResponse, error) {
	taskID := ""
	if in != nil {
		taskID = strings.TrimSpace(in.TaskId)
	}
	if taskID == "" {
		return &pb.AdminGetConsolidationTaskResponse{
			Success:   false,
			Message:   "task_id is required",
			RequestId: resp.RequestID(l.ctx),
			Timestamp: resp.Timestamp(),
		}, nil
	}
	if l.svcCtx == nil || l.svcCtx.ConsolidationRpc == nil {
		return &pb.AdminGetConsolidationTaskResponse{
			Success:   false,
			Message:   "Consolidation服务未配置或未启动",
			RequestId: resp.RequestID(l.ctx),
			Timestamp: resp.Timestamp(),
		}, nil
	}

	rpcResp, err := l.svcCtx.ConsolidationRpc.GetConsolidationTask(l.ctx, &pb.GetConsolidationTaskRequest{TaskId: taskID})
	if err != nil {
		l.Logger.Errorf("call consolidation.GetConsolidationTask failed: %v", err)
		return &pb.AdminGetConsolidationTaskResponse{
			Success:   false,
			Message:   "调用Consolidation服务失败",
			RequestId: resp.RequestID(l.ctx),
			Timestamp: resp.Timestamp(),
		}, nil
	}
	if rpcResp == nil || !rpcResp.Success || rpcResp.Task == nil {
		msg := "not found"
		if rpcResp != nil && rpcResp.Message != "" {
			msg = rpcResp.Message
		}
		return &pb.AdminGetConsolidationTaskResponse{
			Success:   false,
			Message:   msg,
			RequestId: resp.RequestID(l.ctx),
			Timestamp: resp.Timestamp(),
		}, nil
	}

	return &pb.AdminGetConsolidationTaskResponse{
		Success: true,
		Message: "ok",
		Data: &pb.AdminGetConsolidationTaskData{
			Task: toAdminConsolidationTaskItem(rpcResp.Task, newAssetPrecisionResolver(l.ctx, l.svcCtx)(rpcResp.Task.Chain, rpcResp.Task.AssetSymbol)),
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
