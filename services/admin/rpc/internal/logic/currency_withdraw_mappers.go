package logic

import (
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/model"
)

func toPBCurrencyWithdrawalItem(m *model.CurrencyWithdrawOrderModel) *pb.CurrencyWithdrawalItem {
	if m == nil {
		return nil
	}
	txHash := ""
	if m.TxHash != nil {
		txHash = strings.TrimSpace(*m.TxHash)
	}
	errMsg := ""
	if m.ErrorMessage != nil {
		errMsg = strings.TrimSpace(*m.ErrorMessage)
	}
	auditNote := ""
	if m.AuditNote != nil {
		auditNote = strings.TrimSpace(*m.AuditNote)
	}
	transferNote := ""
	if m.TransferNote != nil {
		transferNote = strings.TrimSpace(*m.TransferNote)
	}
	memoTag := ""
	if m.MemoTag != nil {
		memoTag = strings.TrimSpace(*m.MemoTag)
	}
	feeRuleSummary := ""
	if m.FeeRuleSummary != nil {
		feeRuleSummary = strings.TrimSpace(*m.FeeRuleSummary)
	}

	return &pb.CurrencyWithdrawalItem{
		Id:               m.ID,
		UserId:           m.UserID,
		AssetCode:        strings.TrimSpace(m.AssetCode),
		ChainCode:        strings.TrimSpace(m.ChainCode),
		Amount:           strings.TrimSpace(m.Amount),
		Fee:              strings.TrimSpace(m.Fee),
		FromAddress:      strings.TrimSpace(m.FromAddress),
		ToAddress:        strings.TrimSpace(m.ToAddress),
		Strategy:         strings.TrimSpace(m.Strategy),
		Status:           strings.TrimSpace(m.Status),
		TxHash:           txHash,
		ErrorMessage:     errMsg,
		AuditAdminId:     m.AuditAdminID,
		AuditNote:        auditNote,
		AuditedAt:        formatTimePtr(m.AuditedAt),
		TransferAdminId:  m.TransferAdminID,
		TransferNote:     transferNote,
		TransferredAt:    formatTimePtr(m.TransferredAt),
		UpdatedBy:        m.UpdatedBy,
		CreatedAt:        formatTime(m.CreatedAt),
		UpdatedAt:        formatTime(m.UpdatedAt),
		MemoTag:          memoTag,
		CreatedByAdminId: m.CreatedByAdminID,
		FeeRuleSummary:   feeRuleSummary,
		FeeRuleSource:    strings.TrimSpace(m.FeeRuleSource),
	}
}
