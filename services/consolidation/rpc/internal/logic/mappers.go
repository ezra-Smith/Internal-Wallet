package logic

import (
	"encoding/json"

	"internalwallet/proto/pb"
	"internalwallet/services/consolidation/rpc/internal/models"

	"google.golang.org/protobuf/types/known/structpb"
)

func taskStatusToProto(s models.ConsolidationTaskStatus) pb.ConsolidationTaskStatus {
	switch s {
	case models.ConsolidationTaskStatusPending:
		return pb.ConsolidationTaskStatus_CONSOLIDATION_TASK_STATUS_PENDING
	case models.ConsolidationTaskStatusNeedEnergy:
		return pb.ConsolidationTaskStatus_CONSOLIDATION_TASK_STATUS_NEED_ENERGY
	case models.ConsolidationTaskStatusInProgress:
		return pb.ConsolidationTaskStatus_CONSOLIDATION_TASK_STATUS_IN_PROGRESS
	case models.ConsolidationTaskStatusConfirmed:
		return pb.ConsolidationTaskStatus_CONSOLIDATION_TASK_STATUS_CONFIRMED
	case models.ConsolidationTaskStatusFailed:
		return pb.ConsolidationTaskStatus_CONSOLIDATION_TASK_STATUS_FAILED
	case models.ConsolidationTaskStatusTimeout:
		return pb.ConsolidationTaskStatus_CONSOLIDATION_TASK_STATUS_TIMEOUT
	case models.ConsolidationTaskStatusNeedGas:
		return pb.ConsolidationTaskStatus_CONSOLIDATION_TASK_STATUS_NEED_GAS
	case models.ConsolidationTaskStatusNeedBandwidth:
		return pb.ConsolidationTaskStatus_CONSOLIDATION_TASK_STATUS_NEED_BANDWIDTH
	case models.ConsolidationTaskStatusCancelled:
		return pb.ConsolidationTaskStatus_CONSOLIDATION_TASK_STATUS_CANCELLED
	case models.ConsolidationTaskStatusPermanentFailed:
		return pb.ConsolidationTaskStatus_CONSOLIDATION_TASK_STATUS_PERMANENT_FAILED
	default:
		return pb.ConsolidationTaskStatus_CONSOLIDATION_TASK_STATUS_UNSPECIFIED
	}
}

func toProtoTask(t *models.ConsolidationTask) *pb.ConsolidationTask {
	if t == nil {
		return nil
	}
	out := &pb.ConsolidationTask{
		TaskId:      t.TaskID,
		Chain:       t.Chain,
		AssetSymbol: t.AssetSymbol,
		FromAddress: t.FromAddress,
		ToAddress:   t.ToAddress,
		Amount:      t.Amount,
		Status:      taskStatusToProto(t.Status),
		RetryCount:  t.RetryCount,
		CreatedAt:   t.CreatedAt.Unix(),
		UpdatedAt:   t.UpdatedAt.Unix(),
	}
	if t.TokenContract != nil {
		out.TokenContract = *t.TokenContract
	}
	if t.EstimatedFee != nil {
		out.EstimatedFee = *t.EstimatedFee
	}
	if t.ActualFee != nil {
		out.ActualFee = *t.ActualFee
	}
	if t.TxHash != nil {
		out.TxHash = *t.TxHash
	}
	if t.ErrorMessage != nil {
		out.ErrorMessage = *t.ErrorMessage
	}
	if t.StartedAt != nil {
		out.StartedAt = t.StartedAt.Unix()
	}
	if t.ConfirmedAt != nil {
		out.ConfirmedAt = t.ConfirmedAt.Unix()
	}
	if t.EnergyRentalID != nil {
		out.EnergyRentalId = *t.EnergyRentalID
	}
	if t.NextAttemptAt != nil {
		out.NextAttemptAt = t.NextAttemptAt.Unix()
	}
	if t.ClaimedBy != nil {
		out.ClaimedBy = *t.ClaimedBy
	}
	if t.ClaimedUntil != nil {
		out.ClaimedUntil = t.ClaimedUntil.Unix()
	}
	out.EnergyOrderCount = t.EnergyOrderCount
	out.EnergyTotalCostSun = t.EnergyTotalCostSun
	return out
}

func toProtoLog(m *models.ConsolidationLog) *pb.ConsolidationLog {
	if m == nil {
		return nil
	}
	out := &pb.ConsolidationLog{
		Id:        m.ID,
		TaskId:    m.TaskID,
		LogLevel:  m.LogLevel,
		Message:   m.Message,
		CreatedAt: m.CreatedAt.Unix(),
	}
	out.Details = bytesToStruct(m.Details)
	return out
}

func topupStatusToProto(s models.ConsolidationTopUpStatus) pb.ConsolidationTopupStatus {
	switch s {
	case models.ConsolidationTopUpStatusPending:
		return pb.ConsolidationTopupStatus_CONSOLIDATION_TOPUP_STATUS_PENDING
	case models.ConsolidationTopUpStatusSent:
		return pb.ConsolidationTopupStatus_CONSOLIDATION_TOPUP_STATUS_SENT
	case models.ConsolidationTopUpStatusConfirmed:
		return pb.ConsolidationTopupStatus_CONSOLIDATION_TOPUP_STATUS_CONFIRMED
	case models.ConsolidationTopUpStatusFailed:
		return pb.ConsolidationTopupStatus_CONSOLIDATION_TOPUP_STATUS_FAILED
	default:
		return pb.ConsolidationTopupStatus_CONSOLIDATION_TOPUP_STATUS_UNSPECIFIED
	}
}

func toProtoTopup(m *models.ConsolidationTopUpRecord) *pb.ConsolidationTopupRecord {
	if m == nil {
		return nil
	}
	out := &pb.ConsolidationTopupRecord{
		Id:          m.ID,
		TaskId:      m.TaskID,
		Chain:       m.Chain,
		AssetSymbol: m.AssetSymbol,
		FromAddress: m.FromAddress,
		ToAddress:   m.ToAddress,
		Amount:      m.Amount,
		Purpose:     m.Purpose,
		Status:      topupStatusToProto(m.Status),
		RetryCount:  m.RetryCount,
		CreatedAt:   m.CreatedAt.Unix(),
		UpdatedAt:   m.UpdatedAt.Unix(),
	}
	if m.TxHash != nil {
		out.TxHash = *m.TxHash
	}
	if m.ErrorMessage != nil {
		out.ErrorMessage = *m.ErrorMessage
	}
	if m.ConfirmedAt != nil {
		out.ConfirmedAt = m.ConfirmedAt.Unix()
	}
	return out
}

func energyRentalStatusToProto(s models.EnergyRentalStatus) pb.EnergyRentalStatus {
	switch s {
	case models.EnergyRentalStatusPending:
		return pb.EnergyRentalStatus_ENERGY_RENTAL_STATUS_PENDING
	case models.EnergyRentalStatusConfirmed:
		return pb.EnergyRentalStatus_ENERGY_RENTAL_STATUS_CONFIRMED
	case models.EnergyRentalStatusFailed:
		return pb.EnergyRentalStatus_ENERGY_RENTAL_STATUS_FAILED
	case models.EnergyRentalStatusExpired:
		return pb.EnergyRentalStatus_ENERGY_RENTAL_STATUS_EXPIRED
	default:
		return pb.EnergyRentalStatus_ENERGY_RENTAL_STATUS_UNSPECIFIED
	}
}

func toProtoEnergyRental(m *models.EnergyRentalRecord) *pb.EnergyRentalRecord {
	if m == nil {
		return nil
	}
	out := &pb.EnergyRentalRecord{
		Id:              m.ID,
		OrderId:         m.OrderID,
		ReceiverAddress: m.ReceiverAddress,
		EnergyAmount:    m.EnergyAmount,
		RentalDuration:  m.RentalDuration,
		PricePerEnergy:  m.PricePerEnergy,
		TotalCost:       m.TotalCost,
		Provider:        m.Provider,
		Status:          energyRentalStatusToProto(m.Status),
		CreatedAt:       m.CreatedAt.Unix(),
		UpdatedAt:       m.UpdatedAt.Unix(),
	}
	out.ProviderResponse = bytesToStruct(m.ProviderResponse)
	if m.ErrorMessage != nil {
		out.ErrorMessage = *m.ErrorMessage
	}
	if m.ConfirmedAt != nil {
		out.ConfirmedAt = m.ConfirmedAt.Unix()
	}
	return out
}

func bytesToStruct(b []byte) *structpb.Struct {
	if len(b) == 0 {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil
	}
	s, err := structpb.NewStruct(m)
	if err != nil {
		return nil
	}
	return s
}
