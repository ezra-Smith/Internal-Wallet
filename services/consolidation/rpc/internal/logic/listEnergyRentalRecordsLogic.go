package logic

import (
	"context"
	"strings"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/consolidation/rpc/internal/models"
	"internalwallet/services/consolidation/rpc/internal/repository"
	"internalwallet/services/consolidation/rpc/internal/svc"

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

func (l *ListEnergyRentalRecordsLogic) ListEnergyRentalRecords(in *pb.ListEnergyRentalRecordsRequest) (*pb.ListEnergyRentalRecordsResponse, error) {
	if in == nil {
		return &pb.ListEnergyRentalRecordsResponse{Success: false, Message: "request is required"}, nil
	}
	if l.svcCtx == nil || l.svcCtx.EnergyRentalRepo == nil {
		return &pb.ListEnergyRentalRecordsResponse{Success: false, Message: "service not ready"}, nil
	}

	var receiver *string
	if strings.TrimSpace(in.ReceiverAddress) != "" {
		v := strings.TrimSpace(in.ReceiverAddress)
		receiver = &v
	}
	var provider *string
	if strings.TrimSpace(in.Provider) != "" {
		v := strings.TrimSpace(in.Provider)
		provider = &v
	}
	var orderID *string
	if strings.TrimSpace(in.OrderId) != "" {
		v := strings.TrimSpace(in.OrderId)
		orderID = &v
	}

	var status *models.EnergyRentalStatus
	if in.Status != pb.EnergyRentalStatus_ENERGY_RENTAL_STATUS_UNSPECIFIED && in.Status != 0 {
		var s models.EnergyRentalStatus
		switch in.Status {
		case pb.EnergyRentalStatus_ENERGY_RENTAL_STATUS_PENDING:
			s = models.EnergyRentalStatusPending
		case pb.EnergyRentalStatus_ENERGY_RENTAL_STATUS_CONFIRMED:
			s = models.EnergyRentalStatusConfirmed
		case pb.EnergyRentalStatus_ENERGY_RENTAL_STATUS_FAILED:
			s = models.EnergyRentalStatusFailed
		case pb.EnergyRentalStatus_ENERGY_RENTAL_STATUS_EXPIRED:
			s = models.EnergyRentalStatusExpired
		default:
			s = 0
		}
		if s != 0 {
			status = &s
		}
	}

	var from *time.Time
	if in.CreatedAtFrom > 0 {
		v := time.Unix(in.CreatedAtFrom, 0).Local()
		from = &v
	}
	var to *time.Time
	if in.CreatedAtTo > 0 {
		v := time.Unix(in.CreatedAtTo, 0).Local()
		to = &v
	}

	items, total, err := l.svcCtx.EnergyRentalRepo.List(l.ctx, repository.ListEnergyRentalRecordsRequest{
		Page:          in.Page,
		PageSize:      in.PageSize,
		Receiver:      receiver,
		Provider:      provider,
		Status:        status,
		OrderID:       orderID,
		CreatedAtFrom: from,
		CreatedAtTo:   to,
	})
	if err != nil {
		return &pb.ListEnergyRentalRecordsResponse{Success: false, Message: err.Error()}, nil
	}

	resp := &pb.ListEnergyRentalRecordsResponse{
		Success: true,
		Message: "ok",
		Total:   total,
		Records: make([]*pb.EnergyRentalRecord, 0, len(items)),
	}
	for i := range items {
		it := items[i]
		resp.Records = append(resp.Records, toProtoEnergyRental(&it))
	}
	return resp, nil
}
