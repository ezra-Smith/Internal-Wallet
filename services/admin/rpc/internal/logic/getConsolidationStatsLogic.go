package logic

import (
	"context"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

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

func (l *GetConsolidationStatsLogic) GetConsolidationStats(in *pb.AdminGetConsolidationStatsRequest) (*pb.AdminGetConsolidationStatsResponse, error) {
	if in == nil {
		in = &pb.AdminGetConsolidationStatsRequest{}
	}
	if l.svcCtx == nil || l.svcCtx.ConsolidationRpc == nil {
		return &pb.AdminGetConsolidationStatsResponse{
			Success:   false,
			Message:   "Consolidation服务未配置或未启动",
			RequestId: resp.RequestID(l.ctx),
			Timestamp: resp.Timestamp(),
		}, nil
	}

	rpcResp, err := l.svcCtx.ConsolidationRpc.GetConsolidationStats(l.ctx, &pb.GetConsolidationStatsRequest{
		Chain:       in.Chain,
		AssetSymbol: in.AssetSymbol,
		FromTs:      in.FromTs,
		ToTs:        in.ToTs,
	})
	if err != nil {
		l.Logger.Errorf("call consolidation.GetConsolidationStats failed: %v", err)
		return &pb.AdminGetConsolidationStatsResponse{
			Success:   false,
			Message:   "调用Consolidation服务失败",
			RequestId: resp.RequestID(l.ctx),
			Timestamp: resp.Timestamp(),
		}, nil
	}
	if rpcResp == nil || !rpcResp.Success || rpcResp.Stats == nil {
		msg := "调用Consolidation服务失败"
		if rpcResp != nil && rpcResp.Message != "" {
			msg = rpcResp.Message
		}
		return &pb.AdminGetConsolidationStatsResponse{
			Success:   false,
			Message:   msg,
			RequestId: resp.RequestID(l.ctx),
			Timestamp: resp.Timestamp(),
		}, nil
	}

	// Fill readable values only when request specifies a single asset_symbol.
	// Otherwise, keep readable fields empty to avoid misleading cross-asset aggregation.
	var totalAmountReadable, totalEstimatedFeeReadable, totalActualFeeReadable string
	if in != nil && strings.TrimSpace(in.AssetSymbol) != "" {
		p := newAssetPrecisionResolver(l.ctx, l.svcCtx)(in.Chain, in.AssetSymbol)
		totalAmountReadable = formatSmallestUnitDecimal(rpcResp.Stats.TotalAmount, p)
		totalEstimatedFeeReadable = formatSmallestUnitDecimal(rpcResp.Stats.TotalEstimatedFee, p)
		totalActualFeeReadable = formatSmallestUnitDecimal(rpcResp.Stats.TotalActualFee, p)
	}

	return &pb.AdminGetConsolidationStatsResponse{
		Success: true,
		Message: "ok",
		Data: &pb.AdminGetConsolidationStatsData{
			Stats: &pb.AdminConsolidationStats{
				Total:                     rpcResp.Stats.Total,
				Confirmed:                 rpcResp.Stats.Confirmed,
				Failed:                    rpcResp.Stats.Failed,
				SuccessRate:               rpcResp.Stats.SuccessRate,
				TotalAmount:               rpcResp.Stats.TotalAmount,
				TotalEstimatedFee:         rpcResp.Stats.TotalEstimatedFee,
				TotalActualFee:            rpcResp.Stats.TotalActualFee,
				TotalAmountReadable:       totalAmountReadable,
				TotalEstimatedFeeReadable: totalEstimatedFeeReadable,
				TotalActualFeeReadable:    totalActualFeeReadable,
			},
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
