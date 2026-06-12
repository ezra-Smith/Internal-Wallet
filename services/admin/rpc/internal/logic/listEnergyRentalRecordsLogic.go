package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListEnergyRentalRecordsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListEnergyRentalRecordsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListEnergyRentalRecordsLogic {
	return &ListEnergyRentalRecordsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListEnergyRentalRecordsLogic) ListEnergyRentalRecords(in *pb.AdminListEnergyRentalRecordsRequest) (*pb.AdminListEnergyRentalRecordsResponse, error) {
	if in == nil {
		in = &pb.AdminListEnergyRentalRecordsRequest{}
	}
	if l.svcCtx == nil || l.svcCtx.ConsolidationRpc == nil {
		return &pb.AdminListEnergyRentalRecordsResponse{
			Success:   false,
			Message:   "Consolidation服务未配置或未启动",
			RequestId: resp.RequestID(l.ctx),
			Timestamp: resp.Timestamp(),
		}, nil
	}

	rpcResp, err := l.svcCtx.ConsolidationRpc.ListEnergyRentalRecords(l.ctx, &pb.ListEnergyRentalRecordsRequest{
		Page:            in.Page,
		PageSize:        in.PageSize,
		ReceiverAddress: in.ReceiverAddress,
		Provider:        in.Provider,
		Status:          pb.EnergyRentalStatus(in.Status),
		OrderId:         in.OrderId,
		CreatedAtFrom:   in.CreatedAtFrom,
		CreatedAtTo:     in.CreatedAtTo,
	})
	if err != nil {
		l.Logger.Errorf("call consolidation.ListEnergyRentalRecords failed: %v", err)
		return &pb.AdminListEnergyRentalRecordsResponse{
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
		return &pb.AdminListEnergyRentalRecordsResponse{
			Success:   false,
			Message:   msg,
			RequestId: resp.RequestID(l.ctx),
			Timestamp: resp.Timestamp(),
		}, nil
	}

	items := make([]*pb.AdminEnergyRentalRecordItem, 0, len(rpcResp.Records))
	for _, it := range rpcResp.Records {
		items = append(items, toAdminEnergyRentalRecordItem(it))
	}

	return &pb.AdminListEnergyRentalRecordsResponse{
		Success: true,
		Message: "ok",
		Data: &pb.AdminListEnergyRentalRecordsData{
			Records: items,
		},
		Pagination: calcPagination(in.Page, in.PageSize, rpcResp.Total),
		RequestId:  resp.RequestID(l.ctx),
		Timestamp:  resp.Timestamp(),
	}, nil
}
