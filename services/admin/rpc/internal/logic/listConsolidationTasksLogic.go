package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

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

func (l *ListConsolidationTasksLogic) ListConsolidationTasks(in *pb.AdminListConsolidationTasksRequest) (*pb.AdminListConsolidationTasksResponse, error) {
	if in == nil {
		in = &pb.AdminListConsolidationTasksRequest{}
	}
	if l.svcCtx == nil || l.svcCtx.ConsolidationRpc == nil {
		return &pb.AdminListConsolidationTasksResponse{
			Success:   false,
			Message:   "Consolidation服务未配置或未启动",
			RequestId: resp.RequestID(l.ctx),
			Timestamp: resp.Timestamp(),
		}, nil
	}

	req := &pb.ListConsolidationTasksRequest{
		Page:          in.Page,
		PageSize:      in.PageSize,
		Chain:         in.Chain,
		AssetSymbol:   in.AssetSymbol,
		Status:        pb.ConsolidationTaskStatus(in.Status),
		CreatedAtFrom: in.CreatedAtFrom,
		CreatedAtTo:   in.CreatedAtTo,
		TaskId:        in.TaskId,
		FromAddress:   in.FromAddress,
		ToAddress:     in.ToAddress,
		TxHash:        in.TxHash,
	}

	rpcResp, err := l.svcCtx.ConsolidationRpc.ListConsolidationTasks(l.ctx, req)
	if err != nil {
		l.Logger.Errorf("call consolidation.ListConsolidationTasks failed: %v", err)
		return &pb.AdminListConsolidationTasksResponse{
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
		return &pb.AdminListConsolidationTasksResponse{
			Success:   false,
			Message:   msg,
			RequestId: resp.RequestID(l.ctx),
			Timestamp: resp.Timestamp(),
		}, nil
	}

	items := make([]*pb.AdminConsolidationTaskItem, 0, len(rpcResp.Tasks))
	resolvePrecision := newAssetPrecisionResolver(l.ctx, l.svcCtx)
	for _, t := range rpcResp.Tasks {
		p := resolvePrecision(t.Chain, t.AssetSymbol)
		items = append(items, toAdminConsolidationTaskItem(t, p))
	}

	return &pb.AdminListConsolidationTasksResponse{
		Success: true,
		Message: "ok",
		Data: &pb.AdminListConsolidationTasksData{
			Tasks: items,
		},
		Pagination: calcPagination(in.Page, in.PageSize, rpcResp.Total),
		RequestId:  resp.RequestID(l.ctx),
		Timestamp:  resp.Timestamp(),
	}, nil
}
