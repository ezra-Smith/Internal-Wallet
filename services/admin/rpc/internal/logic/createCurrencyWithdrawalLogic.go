package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"internalwallet/common/constants"
	"internalwallet/common/middleware"
	"internalwallet/common/withdrawcalc"
	"internalwallet/common/utils"
	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	admininterceptor "internalwallet/services/admin/rpc/internal/interceptor"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/repository"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"gorm.io/gorm"
)

type CreateCurrencyWithdrawalLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateCurrencyWithdrawalLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateCurrencyWithdrawalLogic {
	return &CreateCurrencyWithdrawalLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func currencyChainTypeForAddressValidation(chainCode string) string {
	switch strings.ToUpper(strings.TrimSpace(chainCode)) {
	case "TRON", "TRX", "TRN":
		return "tron"
	case "ETH", "ETHEREUM":
		return "ethereum"
	case "BSC", "BNB":
		return "bsc"
	case "POLYGON", "MATIC":
		return "polygon"
	case "BTC", "BITCOIN":
		return "bitcoin"
	default:
		return ""
	}
}

func (l *CreateCurrencyWithdrawalLogic) CreateCurrencyWithdrawal(in *pb.CreateCurrencyWithdrawalRequest) (*pb.CreateCurrencyWithdrawalResponse, error) {
	if in == nil || in.UserId <= 0 || strings.TrimSpace(in.AssetCode) == "" || strings.TrimSpace(in.ChainCode) == "" ||
		strings.TrimSpace(in.ToAddress) == "" || strings.TrimSpace(in.Amount) == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid params", map[string]string{
			"user_id":    "required",
			"asset_code": "required",
			"chain_code": "required",
			"to_address": "required",
			"amount":     "required",
		})
	}
	if l.svcCtx == nil || l.svcCtx.DB == nil || l.svcCtx.UserRepo == nil || l.svcCtx.CurrencyWithdrawOrderRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}
	if l.svcCtx.CurrencyWithdrawOrderEventRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "withdraw events repo not configured", nil)
	}
	if l.svcCtx.AccountingRpc == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "ACCOUNTING_NOT_CONFIGURED", "accounting rpc not configured", nil)
	}

	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	assetCode := strings.ToUpper(strings.TrimSpace(in.AssetCode))
	chainCode := strings.ToUpper(strings.TrimSpace(in.ChainCode))
	toAddress := strings.TrimSpace(in.ToAddress)
	fromAddress := strings.TrimSpace(in.FromAddress)
	strategy := strings.ToLower(strings.TrimSpace(in.Strategy))
	if strategy == "" {
		strategy = "manual_manual"
	}
	if strategy != "manual_manual" && strategy != "manual_auto" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_STRATEGY", "invalid strategy", map[string]string{"strategy": "invalid"})
	}
	if strategy == "manual_auto" && fromAddress == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "FROM_ADDRESS_REQUIRED", "from_address required for manual_auto", map[string]string{"from_address": "required"})
	}

	// Validate user exists.
	if _, err := l.svcCtx.UserRepo.FindByID(l.ctx, in.UserId); err != nil {
		return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "USER_NOT_FOUND", "user not found", nil)
	}

	// Enforce withdraw enablement.
	useGlobalWithdrawFee := true
	if l.svcCtx.CurrencySettingsRepo != nil {
		if s, err := l.svcCtx.CurrencySettingsRepo.GetByAssetCode(l.ctx, assetCode); err == nil && s != nil {
			if !s.Web3WithdrawEnabled {
				return nil, errx.New(codes.FailedPrecondition, 409, errx.CodeConflict, "WITHDRAW_DISABLED", "withdraw disabled", nil)
			}
			useGlobalWithdrawFee = s.UseGlobalWithdrawFee
		}
	}
	if l.svcCtx.CurrencyChainSettingsRepo != nil {
		items, err := l.svcCtx.CurrencyChainSettingsRepo.ListByAssetCode(l.ctx, assetCode)
		if err != nil {
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
		}
		var matched *model.CurrencyChainSettingsModel
		for _, it := range items {
			if it == nil {
				continue
			}
			if strings.EqualFold(strings.TrimSpace(it.ChainCode), chainCode) {
				matched = it
				break
			}
		}
		if matched == nil || matched.Status != 1 || !matched.WithdrawEnabled {
			return nil, errx.New(codes.FailedPrecondition, 409, errx.CodeConflict, "NETWORK_NOT_AVAILABLE", "network not available", nil)
		}
		// min_withdraw_amount is mandatory for withdraw creation.
		if matched.MinWithdrawAmount == nil || strings.TrimSpace(*matched.MinWithdrawAmount) == "" {
			return nil, errx.New(codes.FailedPrecondition, 200, errx.CodeInvalidParam, "MIN_WITHDRAW_AMOUNT_NOT_CONFIGURED", "min withdraw amount not configured", map[string]string{"min_withdraw_amount": "required"})
		}
	}
	// Resolve min_withdraw_amount (validated above when repo is configured).
	var minWithdraw decimal.Decimal
	if l.svcCtx.CurrencyChainSettingsRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "currency chain settings repo not configured", nil)
	}
	matchedChainSetting, err := l.svcCtx.CurrencyChainSettingsRepo.FindByAssetAndChain(l.ctx, assetCode, chainCode)
	if err != nil || matchedChainSetting == nil || matchedChainSetting.MinWithdrawAmount == nil || strings.TrimSpace(*matchedChainSetting.MinWithdrawAmount) == "" {
		return nil, errx.New(codes.FailedPrecondition, 200, errx.CodeInvalidParam, "MIN_WITHDRAW_AMOUNT_NOT_CONFIGURED", "min withdraw amount not configured", map[string]string{"min_withdraw_amount": "required"})
	}
	minWithdraw, err = decimal.NewFromString(strings.TrimSpace(*matchedChainSetting.MinWithdrawAmount))
	if err != nil || minWithdraw.LessThanOrEqual(decimal.Zero) {
		return nil, errx.New(codes.FailedPrecondition, 200, errx.CodeInvalidParam, "MIN_WITHDRAW_AMOUNT_NOT_CONFIGURED", "min withdraw amount not configured", map[string]string{"min_withdraw_amount": "invalid"})
	}

	// Validate addresses.
	chainType := currencyChainTypeForAddressValidation(chainCode)
	if err := validateChainAddress(chainType, toAddress); err != nil {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_ADDRESS", "invalid to_address", map[string]string{"to_address": "invalid"})
	}
	if fromAddress != "" {
		if err := validateChainAddress(chainType, fromAddress); err != nil {
			return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_FROM_ADDRESS", "invalid from_address", map[string]string{"from_address": "invalid"})
		}
	}

	amountStr := strings.TrimSpace(in.Amount)
	amt, err := decimal.NewFromString(amountStr)
	if err != nil || !amt.GreaterThan(decimal.Zero) {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_AMOUNT", "invalid amount", map[string]string{"amount": "invalid"})
	}

	// Calculate withdraw fee based on rules and persist a snapshot (for audit transparency).
	feeSource := constants.WithdrawFeeRuleSourceGlobal
	feeRuleItems := make([]withdrawcalc.FeeRuleSnapshotItem, 0)
	if !useGlobalWithdrawFee && l.svcCtx.CurrencyWithdrawFeeRuleRepo != nil {
		if rr, _ := l.svcCtx.CurrencyWithdrawFeeRuleRepo.ListByAssetChain(l.ctx, assetCode, chainCode); len(rr) > 0 {
			feeSource = constants.WithdrawFeeRuleSourceAsset
			for _, r := range rr {
				if r == nil {
					continue
				}
				feeRuleItems = append(feeRuleItems, withdrawcalc.FeeRuleSnapshotItem{
					ID:        r.ID,
					RuleType:  strings.TrimSpace(r.RuleType),
					Value:     strings.TrimSpace(r.Value),
					MinFee:    r.MinFee,
					MaxFee:    r.MaxFee,
					MinAmount: r.MinAmount,
					MaxAmount: r.MaxAmount,
					SortOrder: r.SortOrder,
				})
			}
		}
	}
	if len(feeRuleItems) == 0 && l.svcCtx.CurrencyGlobalWithdrawFeeRuleRepo != nil {
		// NOTE: global fee rules are stored by asset code in chain_code column historically.
		if gr, _ := l.svcCtx.CurrencyGlobalWithdrawFeeRuleRepo.ListByChain(l.ctx, assetCode); len(gr) > 0 {
			feeSource = constants.WithdrawFeeRuleSourceGlobal
			for _, g := range gr {
				if g == nil {
					continue
				}
				feeRuleItems = append(feeRuleItems, withdrawcalc.FeeRuleSnapshotItem{
					ID:        g.ID,
					RuleType:  strings.TrimSpace(g.RuleType),
					Value:     strings.TrimSpace(g.Value),
					MinFee:    g.MinFee,
					MaxFee:    g.MaxFee,
					MinAmount: g.MinAmount,
					MaxAmount: g.MaxAmount,
					SortOrder: g.SortOrder,
				})
			}
		}
	}
	feeDec, feeRuleSummary, feeSnapshot := withdrawcalc.BuildWithdrawFeeSnapshot(amt, amountStr, assetCode, chainCode, feeSource, feeRuleItems)

	// 截断到 6 位小数以匹配 Accounting 服务的精度限制
	amtT := withdrawcalc.TruncateAccounting(amt)
	feeT := withdrawcalc.TruncateAccounting(feeDec)
	minT := withdrawcalc.TruncateAccounting(minWithdraw)
	amountStrTruncated := amtT.String()
	feeStrTruncated := feeT.String()

	// 内扣模式：amount 为总扣款金额；净到账 = amount - fee
	// 强制要求：amount > fee 且 amount >= min_withdraw_amount + fee（等价于净到账 >= min_withdraw_amount）
	if v := withdrawcalc.ValidateGrossAmountForInnerDeduct(amtT, feeT, minT); v != nil {
		switch v.Code {
		case "AMOUNT_MUST_EXCEED_FEE":
			return nil, errx.New(codes.InvalidArgument, 200, errx.CodeInvalidParam, "AMOUNT_MUST_EXCEED_FEE", "amount must exceed fee", map[string]string{"amount": "must exceed fee"})
		case "AMOUNT_BELOW_MIN_PLUS_FEE":
			return nil, errx.NewWithMetadata(codes.InvalidArgument, 200, errx.CodeInvalidParam, "AMOUNT_BELOW_MIN_PLUS_FEE", "amount below minimum withdraw amount plus fee", map[string]string{"amount": "too small"}, v.Meta)
		case "MIN_WITHDRAW_AMOUNT_NOT_CONFIGURED":
			return nil, errx.New(codes.FailedPrecondition, 200, errx.CodeInvalidParam, "MIN_WITHDRAW_AMOUNT_NOT_CONFIGURED", "min withdraw amount not configured", map[string]string{"min_withdraw_amount": "required"})
		default:
			return nil, errx.New(codes.InvalidArgument, 200, errx.CodeInvalidParam, v.Code, v.Message, nil)
		}
	}

	withdrawID := utils.GenerateID()
	bizRef := strconv.FormatInt(withdrawID, 10)

	// 内扣模式：只冻结 amount（手续费从提现金额中扣除，不额外冻结）
	freezeResp, freezeErr := l.svcCtx.AccountingRpc.FreezeWithdraw(l.ctx, &pb.FreezeWithdrawRequest{
		IdempotencyKey:   fmt.Sprintf("withdraw:freeze:%d", withdrawID),
		BizRef:           bizRef,
		UserId:           in.UserId,
		AssetCode:        assetCode,
		AmountDecimal:    amountStrTruncated, // 内扣模式：只冻结 amount
		ChainCode:        chainCode,
		ToAddress:        toAddress,
		PrincipalDecimal: amountStrTruncated,
		FeeDecimal:       feeStrTruncated,
		Memo:             strings.TrimSpace(in.MemoTag),
	})
	if err := errFromAccountingTx("FREEZE_WITHDRAW", freezeResp, freezeErr); err != nil {
		return nil, err
	}

	ip := middleware.GetClientIP(l.ctx)
	ua := middleware.GetUserAgent(l.ctx)
	memoTag := strings.TrimSpace(in.MemoTag)

	var created *model.CurrencyWithdrawOrderModel
	if err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		repo := l.svcCtx.CurrencyWithdrawOrderRepo.WithTx(tx)
		eventRepo := l.svcCtx.CurrencyWithdrawOrderEventRepo.WithTx(tx)
		auditRepo := repository.NewAdminAuditLogRepository(tx)

		var memoPtr *string
		if memoTag != "" {
			memoPtr = &memoTag
		}

		m := &model.CurrencyWithdrawOrderModel{
			UserID:           in.UserId,
			AssetCode:        assetCode,
			ChainCode:        chainCode,
			Amount:           amountStrTruncated,
			Fee:              feeStrTruncated,
			FeeRuleSnapshot:  feeSnapshot,
			FeeRuleSummary:   func() *string { s := strings.TrimSpace(feeRuleSummary); if s == "" { return nil }; return &s }(),
			FeeRuleSource:    strings.TrimSpace(feeSource),
			FromAddress:      fromAddress,
			ToAddress:        toAddress,
			MemoTag:          memoPtr,
			CreatedByAdminID: current.ID,
			Strategy:         strategy,
			Status:           "pending",
			AuditAdminID:     0,
			TransferAdminID:  0,
			UpdatedBy:        current.ID,
		}
		m.ID = withdrawID
		if err := repo.Create(l.ctx, m); err != nil {
			return err
		}
		created = m

		events := []*model.CurrencyWithdrawOrderEventModel{
			newWithdrawOrderEvent(withdrawID, constants.WithdrawEventOrderCreated, constants.WithdrawActorAdmin, current.ID, ip, "Withdrawal order created", map[string]any{
				"user_id":    in.UserId,
				"asset_code": assetCode,
				"chain_code": chainCode,
				"amount":     amountStrTruncated,
				"fee":        feeStrTruncated,
				"strategy":   strategy,
				"to_address": toAddress,
				"from_address": func() string {
					if fromAddress == "" {
						return ""
					}
					return fromAddress
				}(),
				"memo_tag":            memoTag,
				"created_by_admin_id": current.ID,
			}),
			newWithdrawOrderEvent(withdrawID, constants.WithdrawEventAssetFrozen, constants.WithdrawActorSystem, 0, "", "Asset frozen for withdrawal", map[string]any{
				"tx_id":           freezeResp.TxId,
				"duplicate":       freezeResp.Duplicate,
				"asset_code":      assetCode,
				"amount_decimal":  strings.TrimSpace(amountStr), // 内扣模式：冻结金额=提现金额
				"idempotency_key": fmt.Sprintf("withdraw:freeze:%d", withdrawID),
				"biz_ref":         bizRef,
			}),
		}
		if err := eventRepo.CreateMultiple(l.ctx, events); err != nil {
			return err
		}

		detailsBytes, _ := json.Marshal(map[string]interface{}{
			"id":         created.ID,
			"user_id":    in.UserId,
			"asset_code": assetCode,
			"chain_code": chainCode,
			"amount":     amountStrTruncated,
			"fee":        feeStrTruncated,
			"to_address": maskAddress(toAddress),
			"from_address": func() string {
				if fromAddress == "" {
					return ""
				}
				return maskAddress(fromAddress)
			}(),
			"strategy":   strategy,
			"memo_tag":   memoTag,
			"status":     "pending",
			"created_by": current.ID,
		})
		_ = auditRepo.CreateLog(l.ctx, &model.AdminAuditLogModel{
			AdminID:     current.ID,
			Action:      "currency.withdrawal.create",
			TargetType:  "currency_withdraw_order",
			TargetID:    "",
			Description: "创建提现订单",
			Details:     detailsBytes,
			IP:          ip,
			UserAgent:   ua,
		})
		return nil
	}); err != nil {
		// Best-effort unfreeze if DB create failed.
		// 内扣模式：解冻 amount（和冻结金额一致）
		unfreezeResp, unfreezeErr := l.svcCtx.AccountingRpc.UnfreezeWithdraw(l.ctx, &pb.UnfreezeWithdrawRequest{
			IdempotencyKey: fmt.Sprintf("withdraw:unfreeze:%d", withdrawID),
			BizRef:         bizRef,
			UserId:         in.UserId,
			AssetCode:      assetCode,
			AmountDecimal:  strings.TrimSpace(amountStr), // 内扣模式：解冻 amount
			FinalStatus:    "failed",
			Reason:         "create withdrawal failed",
		})
		_ = errFromAccountingTx("UNFREEZE_WITHDRAW", unfreezeResp, unfreezeErr)

		l.Logger.Errorf("create currency withdrawal failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "create withdrawal failed", nil)
	}

	return &pb.CreateCurrencyWithdrawalResponse{
		Success: true,
		Message: "ok",
		Data: &pb.CreateCurrencyWithdrawalData{
			Withdrawal: toPBCurrencyWithdrawalItem(created),
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
