package logic

import (
	"context"
	"encoding/json"
	"strings"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/repository"
)

func parseAccountingNormalSide(s string) (pb.NormalSide, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debit":
		return pb.NormalSide_NORMAL_SIDE_DEBIT, true
	case "credit":
		return pb.NormalSide_NORMAL_SIDE_CREDIT, true
	default:
		return pb.NormalSide_NORMAL_SIDE_UNSPECIFIED, false
	}
}

func formatAccountingNormalSide(side pb.NormalSide) string {
	switch side {
	case pb.NormalSide_NORMAL_SIDE_DEBIT:
		return "debit"
	case pb.NormalSide_NORMAL_SIDE_CREDIT:
		return "credit"
	default:
		return ""
	}
}

func normalizeAccountingAssetCodes(assetCodes []string) []string {
	if len(assetCodes) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(assetCodes))
	out := make([]string, 0, len(assetCodes))
	for _, a := range assetCodes {
		code := normalizeCode(a)
		if code == "" {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		out = append(out, code)
	}
	return out
}

func toAdminAccountingAccountTypeItem(item *pb.AcctAccountType) *pb.AccountingAccountTypeItem {
	if item == nil {
		return nil
	}
	return &pb.AccountingAccountTypeItem{
		Code:        strings.TrimSpace(item.Code),
		Name:        strings.TrimSpace(item.Name),
		NormalSide:  formatAccountingNormalSide(item.NormalSide),
		AssetCodes:  normalizeAccountingAssetCodes(item.AssetCodes),
		Description: strings.TrimSpace(item.Description),
	}
}

func toAdminAccountingUserBalanceItem(item *pb.UserBalanceItem) *pb.AccountingUserBalanceItem {
	if item == nil {
		return nil
	}
	return &pb.AccountingUserBalanceItem{
		AssetCode:    strings.TrimSpace(item.AssetCode),
		Scale:        item.Scale,
		Available:    strings.TrimSpace(item.Available),
		Locked:       strings.TrimSpace(item.Locked),
		AvailableRaw: strings.TrimSpace(item.AvailableRaw),
		LockedRaw:    strings.TrimSpace(item.LockedRaw),
	}
}

func toAdminAccountingLedgerTxItem(item *pb.LedgerTxItem) *pb.AccountingLedgerTxItem {
	if item == nil {
		return nil
	}
	return &pb.AccountingLedgerTxItem{
		TxId:           item.TxId,
		CreatedAt:      strings.TrimSpace(item.CreatedAt),
		OpType:         strings.TrimSpace(item.OpType),
		BizRef:         strings.TrimSpace(item.BizRef),
		IdempotencyKey: strings.TrimSpace(item.IdempotencyKey),
		RequestHash:    strings.TrimSpace(item.RequestHash),
	}
}

func toAdminAccountingLedgerPostingItem(item *pb.LedgerPostingItem) *pb.AccountingLedgerPostingItem {
	if item == nil {
		return nil
	}
	return &pb.AccountingLedgerPostingItem{
		Seq:                    item.Seq,
		AssetCode:              strings.TrimSpace(item.AssetCode),
		Scale:                  item.Scale,
		Bucket:                 strings.TrimSpace(item.Bucket),
		AccountId:              item.AccountId,
		OwnerType:              strings.TrimSpace(item.OwnerType),
		OwnerId:                item.OwnerId,
		AccountTypeCode:        strings.TrimSpace(item.AccountTypeCode),
		AccountTypeName:        strings.TrimSpace(item.AccountTypeName),
		AccountTypeDescription: strings.TrimSpace(item.AccountTypeDescription),
		AccountNormalSide:      strings.TrimSpace(item.AccountNormalSide),
		ChainScope:             strings.TrimSpace(item.ChainScope),
		DebitRaw:               strings.TrimSpace(item.DebitRaw),
		CreditRaw:              strings.TrimSpace(item.CreditRaw),
		DebitDecimal:           strings.TrimSpace(item.DebitDecimal),
		CreditDecimal:          strings.TrimSpace(item.CreditDecimal),
	}
}

func auditAdminAction(ctx context.Context, repo repository.AdminAuditLogRepository, admin *model.AdminUserModel, action, targetType, targetID, description string, details any) {
	if repo == nil || admin == nil {
		return
	}
	detailsBytes, _ := json.Marshal(details)
	_ = repo.CreateLog(ctx, &model.AdminAuditLogModel{
		AdminID:     admin.ID,
		Action:      action,
		TargetType:  targetType,
		TargetID:    targetID,
		Description: description,
		Details:     detailsBytes,
		IP:          middleware.GetClientIP(ctx),
		UserAgent:   middleware.GetUserAgent(ctx),
	})
}
