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

func (l *ListConsolidationTopupRecordsLogic) ListConsolidationTopupRecords(in *pb.ListConsolidationTopupRecordsRequest) (*pb.ListConsolidationTopupRecordsResponse, error) {
	if in == nil {
		return &pb.ListConsolidationTopupRecordsResponse{Success: false, Message: "request is required"}, nil
	}
	if l.svcCtx == nil || l.svcCtx.TopUpRecordRepo == nil {
		return &pb.ListConsolidationTopupRecordsResponse{Success: false, Message: "service not ready"}, nil
	}

	var taskID *string
	if strings.TrimSpace(in.TaskId) != "" {
		v := strings.TrimSpace(in.TaskId)
		taskID = &v
	}
	var chain *string
	if strings.TrimSpace(in.Chain) != "" {
		v := strings.TrimSpace(in.Chain)
		chain = &v
	}
	var asset *string
	if strings.TrimSpace(in.AssetSymbol) != "" {
		v := strings.TrimSpace(in.AssetSymbol)
		asset = &v
	}
	var purpose *string
	if strings.TrimSpace(in.Purpose) != "" {
		v := strings.TrimSpace(in.Purpose)
		purpose = &v
	}
	var toAddr *string
	if strings.TrimSpace(in.ToAddress) != "" {
		v := strings.TrimSpace(in.ToAddress)
		toAddr = &v
	}
	var txHash *string
	if strings.TrimSpace(in.TxHash) != "" {
		v := strings.TrimSpace(in.TxHash)
		txHash = &v
	}

	var status *models.ConsolidationTopUpStatus
	if in.Status != pb.ConsolidationTopupStatus_CONSOLIDATION_TOPUP_STATUS_UNSPECIFIED && in.Status != 0 {
		var s models.ConsolidationTopUpStatus
		switch in.Status {
		case pb.ConsolidationTopupStatus_CONSOLIDATION_TOPUP_STATUS_PENDING:
			s = models.ConsolidationTopUpStatusPending
		case pb.ConsolidationTopupStatus_CONSOLIDATION_TOPUP_STATUS_SENT:
			s = models.ConsolidationTopUpStatusSent
		case pb.ConsolidationTopupStatus_CONSOLIDATION_TOPUP_STATUS_CONFIRMED:
			s = models.ConsolidationTopUpStatusConfirmed
		case pb.ConsolidationTopupStatus_CONSOLIDATION_TOPUP_STATUS_FAILED:
			s = models.ConsolidationTopUpStatusFailed
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

	items, total, err := l.svcCtx.TopUpRecordRepo.List(l.ctx, repository.ListTopupRecordsRequest{
		Page:          in.Page,
		PageSize:      in.PageSize,
		TaskID:        taskID,
		Chain:         chain,
		AssetSymbol:   asset,
		Purpose:       purpose,
		Status:        status,
		ToAddress:     toAddr,
		TxHash:        txHash,
		CreatedAtFrom: from,
		CreatedAtTo:   to,
	})
	if err != nil {
		return &pb.ListConsolidationTopupRecordsResponse{Success: false, Message: err.Error()}, nil
	}

	resp := &pb.ListConsolidationTopupRecordsResponse{
		Success: true,
		Message: "ok",
		Total:   total,
		Records: make([]*pb.ConsolidationTopupRecord, 0, len(items)),
	}
	for i := range items {
		it := items[i]
		resp.Records = append(resp.Records, toProtoTopup(&it))
	}
	return resp, nil
}
