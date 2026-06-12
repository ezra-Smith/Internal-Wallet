package logic

import (
	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/model"
)

func toPBVaultAdjustmentItem(m *model.VaultAdjustmentModel) *pb.VaultAdjustmentItem {
	if m == nil {
		return nil
	}

	var reviewedBy int64
	if m.ReviewedBy != nil {
		reviewedBy = *m.ReviewedBy
	}
	return &pb.VaultAdjustmentItem{
		Id:             m.ID,
		Network:        m.Network,
		ChainId:        m.ChainID,
		Currency:       m.Currency,
		AdjustmentType: m.AdjustmentType,
		Amount:         m.Amount,
		AmountRaw:      m.AmountRaw,
		AmountUsd:      m.AmountUSD,
		Reason:         m.Reason,
		Source:         m.Source,
		SourceAddress:  derefString(m.SourceAddress),
		TxHash:         derefString(m.TxHash),
		Status:         m.Status,
		SubmittedBy:    m.SubmittedBy,
		SubmittedAt:    formatTimePtr(m.CreatedAt),
		ReviewedBy:     reviewedBy,
		ReviewedAt:     formatTimePtr(m.ReviewedAt),
		ReviewNote:     derefString(m.ReviewNote),
		CompletedAt:    formatTimePtr(m.CompletedAt),
	}
}
