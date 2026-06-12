package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListConsolidationTopupRecordsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListConsolidationTopupRecordsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListConsolidationTopupRecordsLogic {
	return &ListConsolidationTopupRecordsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListConsolidationTopupRecordsLogic) ListConsolidationTopupRecords(in *pb.AdminListConsolidationTopupRecordsRequest) (*pb.AdminListConsolidationTopupRecordsResponse, error) {
	if in == nil {
		in = &pb.AdminListConsolidationTopupRecordsRequest{}
	}
	if l.svcCtx == nil || l.svcCtx.ConsolidationRpc == nil {
		return &pb.AdminListConsolidationTopupRecordsResponse{
			Success:   false,
			Message:   "Consolidation服务未配置或未启动",
			RequestId: resp.RequestID(l.ctx),
			Timestamp: resp.Timestamp(),
		}, nil
	}

	rpcResp, err := l.svcCtx.ConsolidationRpc.ListConsolidationTopupRecords(l.ctx, &pb.ListConsolidationTopupRecordsRequest{
		Page:          in.Page,
		PageSize:      in.PageSize,
		TaskId:        in.TaskId,
		Chain:         in.Chain,
		AssetSymbol:   in.AssetSymbol,
		Purpose:       in.Purpose,
		Status:        pb.ConsolidationTopupStatus(in.Status),
		ToAddress:     in.ToAddress,
		TxHash:        in.TxHash,
		CreatedAtFrom: in.CreatedAtFrom,
		CreatedAtTo:   in.CreatedAtTo,
	})
	if err != nil {
		l.Logger.Errorf("call consolidation.ListConsolidationTopupRecords failed: %v", err)
		return &pb.AdminListConsolidationTopupRecordsResponse{
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
		return &pb.AdminListConsolidationTopupRecordsResponse{
			Success:   false,
			Message:   msg,
			RequestId: resp.RequestID(l.ctx),
			Timestamp: resp.Timestamp(),
		}, nil
	}

	items := make([]*pb.AdminConsolidationTopupRecordItem, 0, len(rpcResp.Records))
	resolvePrecision := newAssetPrecisionResolver(l.ctx, l.svcCtx)
	for _, it := range rpcResp.Records {
		p := resolvePrecision(it.Chain, it.AssetSymbol)
		items = append(items, toAdminConsolidationTopupRecordItem(it, p))
	}

	return &pb.AdminListConsolidationTopupRecordsResponse{
		Success: true,
		Message: "ok",
		Data: &pb.AdminListConsolidationTopupRecordsData{
			Records: items,
		},
		Pagination: calcPagination(in.Page, in.PageSize, rpcResp.Total),
		RequestId:  resp.RequestID(l.ctx),
		Timestamp:  resp.Timestamp(),
	}, nil
}
