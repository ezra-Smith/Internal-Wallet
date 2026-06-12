package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

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

func (l *ListConsolidationLogsLogic) ListConsolidationLogs(in *pb.AdminListConsolidationLogsRequest) (*pb.AdminListConsolidationLogsResponse, error) {
	if in == nil {
		in = &pb.AdminListConsolidationLogsRequest{}
	}
	if l.svcCtx == nil || l.svcCtx.ConsolidationRpc == nil {
		return &pb.AdminListConsolidationLogsResponse{
			Success:   false,
			Message:   "Consolidation服务未配置或未启动",
			RequestId: resp.RequestID(l.ctx),
			Timestamp: resp.Timestamp(),
		}, nil
	}

	rpcResp, err := l.svcCtx.ConsolidationRpc.ListConsolidationLogs(l.ctx, &pb.ListConsolidationLogsRequest{
		Page:          in.Page,
		PageSize:      in.PageSize,
		TaskId:        in.TaskId,
		LogLevel:      in.LogLevel,
		CreatedAtFrom: in.CreatedAtFrom,
		CreatedAtTo:   in.CreatedAtTo,
	})
	if err != nil {
		l.Logger.Errorf("call consolidation.ListConsolidationLogs failed: %v", err)
		return &pb.AdminListConsolidationLogsResponse{
			Success:   false,
			Message:   "调用Consolidation服务失败",
			RequestId: resp.RequestID(l.ctx),
			Timestamp: resp.Timestamp(),
		}, nil
	}
	if rpcResp == nil || !rpcResp.Success {
		msg := "调用Consolidation服务失败"
		if rpcResp != nil && rpcResp.Message != "" {
			msg = rpcResp.Message
		}
		return &pb.AdminListConsolidationLogsResponse{
			Success:   false,
			Message:   msg,
			RequestId: resp.RequestID(l.ctx),
			Timestamp: resp.Timestamp(),
		}, nil
	}

	items := make([]*pb.AdminConsolidationLogItem, 0, len(rpcResp.Logs))
	for _, it := range rpcResp.Logs {
		items = append(items, toAdminConsolidationLogItem(it))
	}

	return &pb.AdminListConsolidationLogsResponse{
		Success: true,
		Message: "ok",
		Data: &pb.AdminListConsolidationLogsData{
			Logs: items,
		},
		Pagination: calcPagination(in.Page, in.PageSize, rpcResp.Total),
		RequestId:  resp.RequestID(l.ctx),
		Timestamp:  resp.Timestamp(),
	}, nil
}
