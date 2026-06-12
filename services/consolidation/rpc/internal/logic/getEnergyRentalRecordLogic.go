package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/consolidation/rpc/internal/svc"

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

func (l *GetEnergyRentalRecordLogic) GetEnergyRentalRecord(in *pb.GetEnergyRentalRecordRequest) (*pb.GetEnergyRentalRecordResponse, error) {
	if in == nil || in.Id <= 0 {
		return &pb.GetEnergyRentalRecordResponse{Success: false, Message: "id is required"}, nil
	}
	if l.svcCtx == nil || l.svcCtx.EnergyRentalRepo == nil {
		return &pb.GetEnergyRentalRecordResponse{Success: false, Message: "service not ready"}, nil
	}

	rec, err := l.svcCtx.EnergyRentalRepo.GetByID(l.ctx, in.Id)
	if err != nil {
		return &pb.GetEnergyRentalRecordResponse{Success: false, Message: err.Error()}, nil
	}
	return &pb.GetEnergyRentalRecordResponse{
		Success: true,
		Message: "ok",
		Record:  toProtoEnergyRental(rec),
	}, nil
}
