// DEPRECATED: This file is no longer in use. The swap statistics feature has been removed.
// The code below is preserved for reference only.

/*
package logic

import (
	"context"
	"fmt"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/swap/rpc/internal/interceptor"
	"internalwallet/services/swap/rpc/internal/repository"
	"internalwallet/services/swap/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetSwapStatisticsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetSwapStatisticsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSwapStatisticsLogic {
	return &GetSwapStatisticsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ==================== 统计与查询 (API Key authenticated) ====================
func (l *GetSwapStatisticsLogic) GetSwapStatistics(in *pb.SwapGetSwapStatisticsRequest) (*pb.SwapGetSwapStatisticsResponse, error) {
	if l.svcCtx.TxRepo == nil {
		return &pb.SwapGetSwapStatisticsResponse{
			Success: false,
			Message: "transaction repository not configured",
		}, nil
	}

	// Get project name from authenticated context
	projectName := interceptor.GetProjectName(l.ctx)

	// Parse date range
	startDate := in.StartDate
	endDate := in.EndDate

	// If period is specified, calculate date range
	if in.Period != "" {
		endTime := time.Now()
		var startTime time.Time
		switch in.Period {
		case "7d":
			startTime = endTime.AddDate(0, 0, -7)
		case "30d":
			startTime = endTime.AddDate(0, 0, -30)
		case "90d":
			startTime = endTime.AddDate(0, 0, -90)
		default:
			startTime = endTime.AddDate(0, 0, -30) // default 30 days
		}
		startDate = startTime.Format("2006-01-02")
		endDate = endTime.Format("2006-01-02")
	}

	// Get summary statistics
	summary, err := l.svcCtx.TxRepo.GetStatsSummary(l.ctx, projectName, startDate, endDate)
	if err != nil {
		logx.Errorf("Get stats summary failed: %v", err)
		return &pb.SwapGetSwapStatisticsResponse{
			Success: false,
			Message: fmt.Sprintf("query failed: %v", err),
		}, nil
	}

	// Get statistics by chain
	byChain, err := l.svcCtx.TxRepo.GetStatsByChain(l.ctx, projectName, startDate, endDate)
	if err != nil {
		logx.Errorf("Get stats by chain failed: %v", err)
		byChain = []*repository.SwapTransactionStatsByChain{}
	}

	// Get statistics by provider
	byProvider, err := l.svcCtx.TxRepo.GetStatsByProvider(l.ctx, projectName, startDate, endDate)
	if err != nil {
		logx.Errorf("Get stats by provider failed: %v", err)
		byProvider = []*repository.SwapTransactionStatsByProvider{}
	}

	// Get trend data
	trend, err := l.svcCtx.TxRepo.GetStatsTrend(l.ctx, projectName, startDate, endDate)
	if err != nil {
		logx.Errorf("Get stats trend failed: %v", err)
		trend = []*repository.SwapTransactionStatsTrend{}
	}

	// Convert to protobuf
	pbByChain := make([]*pb.SwapSvcStatisticsByChain, 0, len(byChain))
	for _, item := range byChain {
		pbByChain = append(pbByChain, &pb.SwapSvcStatisticsByChain{
			ChainId:   item.ChainID,
			ChainName: item.ChainName,
			SwapCount: item.SwapCount,
			VolumeUsd: item.VolumeUSD,
		})
	}

	pbByProvider := make([]*pb.SwapSvcStatisticsByProvider, 0, len(byProvider))
	for _, item := range byProvider {
		pbByProvider = append(pbByProvider, &pb.SwapSvcStatisticsByProvider{
			Provider:    item.Provider,
			SwapCount:   item.SwapCount,
			VolumeUsd:   item.VolumeUSD,
			SuccessRate: item.SuccessRate,
		})
	}

	pbTrend := make([]*pb.SwapSvcStatisticsTrend, 0, len(trend))
	for _, item := range trend {
		pbTrend = append(pbTrend, &pb.SwapSvcStatisticsTrend{
			Date:      item.Date,
			SwapCount: item.SwapCount,
			VolumeUsd: item.VolumeUSD,
		})
	}

	return &pb.SwapGetSwapStatisticsResponse{
		Success: true,
		Message: "success",
		Summary: &pb.SwapSvcStatisticsSummary{
			TotalSwaps:       summary.TotalSwaps,
			TotalVolumeUsd:   summary.TotalVolumeUSD,
			TotalFeeUsd:      summary.TotalFeeUSD,
			AvgSwapAmountUsd: summary.AvgSwapAmountUSD,
			SuccessRate:      summary.SuccessRate,
			UniqueWallets:    summary.UniqueWallets,
		},
		ByChain:    pbByChain,
		ByProvider: pbByProvider,
		Trend:      pbTrend,
	}, nil
}
*/

package logic
