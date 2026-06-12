package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetEnergyRentalRecordLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetEnergyRentalRecordLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetEnergyRentalRecordLogic {
	return &GetEnergyRentalRecordLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetEnergyRentalRecordLogic) GetEnergyRentalRecord(in *pb.AdminGetEnergyRentalRecordRequest) (*pb.AdminGetEnergyRentalRecordResponse, error) {
	if in == nil || in.Id <= 0 {
		return &pb.AdminGetEnergyRentalRecordResponse{
			Success:   false,
			Message:   "id is required",
			RequestId: resp.RequestID(l.ctx),
			Timestamp: resp.Timestamp(),
		}, nil
	}
	if l.svcCtx == nil || l.svcCtx.ConsolidationRpc == nil {
		return &pb.AdminGetEnergyRentalRecordResponse{
			Success:   false,
			Message:   "Consolidation服务未配置或未启动",
			RequestId: resp.RequestID(l.ctx),
			Timestamp: resp.Timestamp(),
		}, nil
	}

	rpcResp, err := l.svcCtx.ConsolidationRpc.GetEnergyRentalRecord(l.ctx, &pb.GetEnergyRentalRecordRequest{Id: in.Id})
	if err != nil {
		l.Logger.Errorf("call consolidation.GetEnergyRentalRecord failed: %v", err)
		return &pb.AdminGetEnergyRentalRecordResponse{
			Success:   false,
			Message:   "调用Consolidation服务失败",
			RequestId: resp.RequestID(l.ctx),
			Timestamp: resp.Timestamp(),
		}, nil
	}
	if rpcResp == nil || !rpcResp.Success || rpcResp.Record == nil {
		msg := "not found"
		if rpcResp != nil && rpcResp.Message != "" {
			msg = rpcResp.Message
		}
		return &pb.AdminGetEnergyRentalRecordResponse{
			Success:   false,
			Message:   msg,
			RequestId: resp.RequestID(l.ctx),
			Timestamp: resp.Timestamp(),
		}, nil
	}

	return &pb.AdminGetEnergyRentalRecordResponse{
		Success: true,
		Message: "ok",
		Data: &pb.AdminGetEnergyRentalRecordData{
			Record: toAdminEnergyRentalRecordItem(rpcResp.Record),
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
