package logic

import (
	"context"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/consolidation/rpc/internal/repository"
	"internalwallet/services/consolidation/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetConsolidationStatsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetConsolidationStatsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetConsolidationStatsLogic {
	return &GetConsolidationStatsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Aggregated stats.
func (l *GetConsolidationStatsLogic) GetConsolidationStats(in *pb.GetConsolidationStatsRequest) (*pb.GetConsolidationStatsResponse, error) {
	if in == nil {
		return &pb.GetConsolidationStatsResponse{Success: false, Message: "request is required"}, nil
	}
	if l.svcCtx == nil || l.svcCtx.ConsolidationTaskRepo == nil {
		return &pb.GetConsolidationStatsResponse{Success: false, Message: "service not ready"}, nil
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
	var from *time.Time
	if in.FromTs > 0 {
		v := time.Unix(in.FromTs, 0).Local()
		from = &v
	}
	var to *time.Time
	if in.ToTs > 0 {
		v := time.Unix(in.ToTs, 0).Local()
		to = &v
	}

	stats, err := l.svcCtx.ConsolidationTaskRepo.GetStats(l.ctx, repository.StatsRequest{
		Chain: chain,
		Asset: asset,
		From:  from,
		To:    to,
	})
	if err != nil {
		return &pb.GetConsolidationStatsResponse{Success: false, Message: err.Error()}, nil
	}

	successRate := 0.0
	if stats.Total > 0 {
		successRate = float64(stats.Confirmed) / float64(stats.Total)
	}

	return &pb.GetConsolidationStatsResponse{
		Success: true,
		Message: "ok",
		Stats: &pb.ConsolidationStats{
			Total:             stats.Total,
			Confirmed:         stats.Confirmed,
			Failed:            stats.Failed,
			SuccessRate:       successRate,
			TotalAmount:       stats.TotalAmount,
			TotalEstimatedFee: stats.TotalEstimatedFee,
			TotalActualFee:    stats.TotalActualFee,
		},
	}, nil
}
