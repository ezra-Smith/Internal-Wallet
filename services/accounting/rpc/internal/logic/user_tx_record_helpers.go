package logic

import (
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/accounting/rpc/internal/model"
)

func toPBUserTxRecordItem(m *model.AcctUserTransactionRecordModel) *pb.UserTransactionRecordItem {
	if m == nil {
		return nil
	}

	timestamp := m.CreatedAt.Unix()
	createdAt := m.CreatedAt.Format("2006-01-02 15:04:05")
	updatedAt := m.UpdatedAt.Format("2006-01-02 15:04:05")

	return &pb.UserTransactionRecordItem{
		Id:                m.ID,
		UserId:            m.UserID,
		TxType:            m.TxType,
		AssetCode:         strings.TrimSpace(m.AssetCode),
		ChainCode:         strings.TrimSpace(m.ChainCode),
		AmountDecimal:     strings.TrimSpace(m.AmountDecimal),
		FeeDecimal:        strings.TrimSpace(m.FeeDecimal),
		Status:            strings.TrimSpace(m.Status),
		Memo:              strings.TrimSpace(m.Memo),
		FromAddress:       strings.TrimSpace(m.FromAddress),
		ToAddress:         strings.TrimSpace(m.ToAddress),
		TxHash:            strings.TrimSpace(m.TxHash),
		Timestamp:         timestamp,
		FreezeLedgerTxId:  derefInt64(m.FreezeLedgerTxID),
		SettleLedgerTxId:  derefInt64(m.SettleLedgerTxID),
		ConfirmLedgerTxId: derefInt64(m.ConfirmLedgerTxID),
		BizRef:            strings.TrimSpace(m.BizRef),
		IdempotencyKey:    strings.TrimSpace(m.IdempotencyKey),
		CreatedAt:         createdAt,
		UpdatedAt:         updatedAt,
	}
}

func derefInt64(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}
