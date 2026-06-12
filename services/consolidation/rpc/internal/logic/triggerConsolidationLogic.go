package logic

import (
	"context"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/consolidation/rpc/internal/scheduler"
	"internalwallet/services/consolidation/rpc/internal/svc"

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

// Manually trigger discovery/execution for a specific chain/asset (best-effort).
func (l *TriggerConsolidationLogic) TriggerConsolidation(in *pb.TriggerConsolidationRequest) (*pb.TriggerConsolidationResponse, error) {
	if in == nil {
		return &pb.TriggerConsolidationResponse{Success: false, Message: "request is required"}, nil
	}
	if l.svcCtx == nil || l.svcCtx.ConsolidationTaskRepo == nil || l.svcCtx.AddressRepo == nil || l.svcCtx.Chain == nil {
		return &pb.TriggerConsolidationResponse{Success: false, Message: "service not ready"}, nil
	}

	start := time.Now()
	l.Infow("manual consolidation trigger requested",
		logx.Field("event", "manual_trigger"),
		logx.Field("chain", in.Chain),
		logx.Field("asset_symbol", in.AssetSymbol),
		logx.Field("from_address", in.FromAddress),
		logx.Field("dry_run", in.DryRun),
	)
	res, err := scheduler.NewConsolidationScheduler(l.svcCtx).DiscoverOnce(l.ctx, scheduler.DiscoveryOptions{
		Chain:       in.Chain,
		AssetSymbol: in.AssetSymbol,
		FromAddress: in.FromAddress,
		DryRun:      in.DryRun,
	})
	if err != nil {
		l.Errorw("manual consolidation trigger failed",
			logx.Field("event", "manual_trigger_failed"),
			logx.Field("chain", in.Chain),
			logx.Field("asset_symbol", in.AssetSymbol),
			logx.Field("from_address", in.FromAddress),
			logx.Field("dry_run", in.DryRun),
			logx.Field("duration_ms", time.Since(start).Milliseconds()),
			logx.Field("error", err),
		)
		return &pb.TriggerConsolidationResponse{Success: false, Message: err.Error()}, nil
	}

	out := &pb.TriggerConsolidationResponse{
		Success:           true,
		Message:           "ok",
		TotalCandidates:   res.TotalCandidates,
		TotalTasksCreated: res.TasksCreated,
	}
	out.Tasks = make([]*pb.ConsolidationTask, 0, len(res.Tasks))
	for i := range res.Tasks {
		task := res.Tasks[i]
		out.Tasks = append(out.Tasks, toProtoTask(&task))
	}
	l.Infow("manual consolidation trigger done",
		logx.Field("event", "manual_trigger_done"),
		logx.Field("chain", in.Chain),
		logx.Field("asset_symbol", in.AssetSymbol),
		logx.Field("from_address", in.FromAddress),
		logx.Field("dry_run", in.DryRun),
		logx.Field("duration_ms", time.Since(start).Milliseconds()),
		logx.Field("total_candidates", out.TotalCandidates),
		logx.Field("tasks_created", out.TotalTasksCreated),
		logx.Field("tasks_returned", len(out.Tasks)),
	)
	return out, nil
}
