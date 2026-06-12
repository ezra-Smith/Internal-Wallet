package logic

import (
	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/model"
)

func toPBTransferBatchItem(m *model.TransferBatchModel) *pb.TransferBatchItem {
	if m == nil {
		return nil
	}
	desc := ""
	if m.Description != nil {
		desc = *m.Description
	}
	approvedBy := ""
	if m.ApprovedBy != nil {
		approvedBy = *m.ApprovedBy
	}
	cancelledBy := ""
	if m.CancelledBy != nil {
		cancelledBy = *m.CancelledBy
	}
	return &pb.TransferBatchItem{
		BatchId:     m.BatchID,
		Name:        m.Name,
		Description: desc,
		// Internal-ledger only; network is fixed.
		Network:         "INTERNAL",
		Currency:        m.Currency,
		TotalRecipients: m.TotalRecipients,
		TotalAmount:     m.TotalAmount,
		TotalAmountUsd:  m.TotalAmountUSD,
		Status:          m.Status,
		SuccessCount:    m.SuccessCount,
		FailedCount:     m.FailedCount,
		PendingCount:    pendingCountFromBatch(m),
		CreatedBy:       m.CreatedBy,
		CreatedAt:       formatTimePtr(m.CreatedAt),
		SubmittedAt:     formatTimePtr(m.SubmittedAt),
		ApprovedBy:      approvedBy,
		ApprovedAt:      formatTimePtr(m.ApprovedAt),
		ProcessedAt:     formatTimePtr(m.ProcessedAt),
		CompletedAt:     formatTimePtr(m.CompletedAt),
		CancelledAt:     formatTimePtr(m.CancelledAt),
		CancelledBy:     cancelledBy,
		TransferType:    m.TransferType,
	}
}

func pendingCountFromBatch(m *model.TransferBatchModel) int32 {
	if m == nil {
		return 0
	}
	p := m.TotalRecipients - m.SuccessCount - m.FailedCount
	if p < 0 {
		return 0
	}
	return p
}

func toPBTransferRecipientItem(m *model.TransferBatchItemModel) *pb.TransferRecipientItem {
	if m == nil {
		return nil
	}
	note := ""
	if m.Note != nil {
		note = *m.Note
	}
	errMsg := ""
	if m.ErrorMessage != nil {
		errMsg = *m.ErrorMessage
	}
	return &pb.TransferRecipientItem{
		Id: m.ID,
		// Internal-ledger only: recipients are UID-based; do not expose "address" to avoid confusion.
		Amount:       m.Amount,
		Note:         note,
		Status:       m.Status,
		ErrorMessage: errMsg,
		RetryCount:   m.RetryCount,
		ProcessedAt:  formatTimePtr(m.ProcessedAt),
		Currency:     m.Currency,
	}
}
