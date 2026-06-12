package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type TriggerConsolidationLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewTriggerConsolidationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TriggerConsolidationLogic {
	return &TriggerConsolidationLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *TriggerConsolidationLogic) TriggerConsolidation(in *pb.AdminTriggerConsolidationRequest) (*pb.AdminTriggerConsolidationResponse, error) {
	if in == nil {
		in = &pb.AdminTriggerConsolidationRequest{}
	}
	if l.svcCtx == nil || l.svcCtx.ConsolidationRpc == nil {
		return &pb.AdminTriggerConsolidationResponse{
			Success:   false,
			Message:   "Consolidation服务未配置或未启动",
			RequestId: resp.RequestID(l.ctx),
			Timestamp: resp.Timestamp(),
		}, nil
	}

	rpcResp, err := l.svcCtx.ConsolidationRpc.TriggerConsolidation(l.ctx, &pb.TriggerConsolidationRequest{
		Chain:       in.Chain,
		AssetSymbol: in.AssetSymbol,
		FromAddress: in.FromAddress,
		DryRun:      in.DryRun,
	})
	if err != nil {
		l.Logger.Errorf("call consolidation.TriggerConsolidation failed: %v", err)
		return &pb.AdminTriggerConsolidationResponse{
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
		return &pb.AdminTriggerConsolidationResponse{
			Success:   false,
			Message:   msg,
			RequestId: resp.RequestID(l.ctx),
			Timestamp: resp.Timestamp(),
		}, nil
	}

	tasks := make([]*pb.AdminConsolidationTaskItem, 0, len(rpcResp.Tasks))
	resolvePrecision := newAssetPrecisionResolver(l.ctx, l.svcCtx)
	for _, t := range rpcResp.Tasks {
		p := resolvePrecision(t.Chain, t.AssetSymbol)
		tasks = append(tasks, toAdminConsolidationTaskItem(t, p))
	}

	return &pb.AdminTriggerConsolidationResponse{
		Success: true,
		Message: "ok",
		Data: &pb.AdminTriggerConsolidationData{
			TotalCandidates:   rpcResp.TotalCandidates,
			TotalTasksCreated: rpcResp.TotalTasksCreated,
			Tasks:             tasks,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
