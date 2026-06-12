package logic

import (
	"context"
	"strings"

	"internalwallet/common/withdrawcalc"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
)

// EstimateWithdrawFeeLogic Web2提币手续费预估（不冻结资金、不创建订单）
type EstimateWithdrawFeeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewEstimateWithdrawFeeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EstimateWithdrawFeeLogic {
	return &EstimateWithdrawFeeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *EstimateWithdrawFeeLogic) EstimateWithdrawFee(in *pb.EstimateWithdrawFeeReq) (*pb.EstimateWithdrawFeeResp, error) {
	if in == nil || strings.TrimSpace(in.Asset) == "" || strings.TrimSpace(in.Chain) == "" {
		return nil, errx.InvalidParam("invalid params")
	}

	assetCode := normalizeCode(in.Asset)
	chainCode := normalizeCode(in.Chain)

	// Ensure withdraw is enabled for this asset/chain (same gate as CreateWithdraw).
	eff := defaultEffectiveCurrencySettings()
	if l.svcCtx.CurrencySettingsRepository != nil {
		if s, err := l.svcCtx.CurrencySettingsRepository.GetByAssetCode(l.ctx, assetCode); err == nil && s != nil {
			eff.Web3DepositEnabled = s.Web3DepositEnabled
			eff.Web3WithdrawEnabled = s.Web3WithdrawEnabled
			eff.UseGlobalWithdrawFee = s.UseGlobalWithdrawFee
			eff.UseGlobalWithdrawAudit = s.UseGlobalWithdrawAudit
		}
	}
	if !eff.Web3WithdrawEnabled {
		return nil, errx.WithdrawDisabled()
	}

	// min_withdraw_amount is mandatory (same as CreateWithdraw).
	if l.svcCtx.CurrencyChainSettingsRepository == nil {
		return nil, errx.Internal("currency chain settings not configured")
	}
	cs, err := l.svcCtx.CurrencyChainSettingsRepository.FindByAssetChain(l.ctx, assetCode, chainCode)
	if err != nil || cs == nil {
		return nil, errx.MinWithdrawAmountNotConfigured(assetCode, chainCode)
	}
	if cs.Status != 1 || !cs.WithdrawEnabled {
		return nil, errx.NetworkNotAvailable()
	}
	if cs.MinWithdrawAmount == nil || strings.TrimSpace(*cs.MinWithdrawAmount) == "" {
		return nil, errx.MinWithdrawAmountNotConfigured(assetCode, chainCode)
	}
	minWithdrawStr := strings.TrimSpace(*cs.MinWithdrawAmount)
	minWithdrawDec, minErr := decimal.NewFromString(minWithdrawStr)
	if minErr != nil || minWithdrawDec.LessThanOrEqual(decimal.Zero) {
		return nil, errx.MinWithdrawAmountNotConfigured(assetCode, chainCode)
	}

	// Load fee rules (aligned with CreateWithdraw).
	feeSource, ruleItems := LoadWithdrawFeeRuleItems(l.ctx, l.svcCtx, assetCode, chainCode, eff.UseGlobalWithdrawFee)
	// Summary for UI (same as CreateWithdraw snapshot summary).
	_, feeRuleSummary, _ := withdrawcalc.BuildWithdrawFeeSnapshot(decimal.Zero, "0", assetCode, chainCode, feeSource, ruleItems)

	feeStr := "0"
	var matchedMinAmount, matchedMaxAmount string // keep for backward UI usage
	if strings.TrimSpace(in.Amount) != "" {
		amt, err := decimal.NewFromString(strings.TrimSpace(in.Amount))
		if err != nil {
			return nil, errx.InvalidParam("INVALID_AMOUNT_FORMAT")
		}
		if !amt.GreaterThan(decimal.Zero) {
			return nil, errx.InvalidParam("AMOUNT_MUST_BE_POSITIVE")
		}
		feeDec, _, _ := withdrawcalc.BuildWithdrawFeeSnapshot(amt, amt.String(), assetCode, chainCode, feeSource, ruleItems)
		feeStr = feeDec.String()

		// Validate with the same rules as CreateWithdraw:
		// amount is gross (inner deduct), require amount > fee and amount >= min + fee.
		amtT := withdrawcalc.TruncateAccounting(amt)
		feeT := withdrawcalc.TruncateAccounting(feeDec)
		minT := withdrawcalc.TruncateAccounting(minWithdrawDec)
		if v := withdrawcalc.ValidateGrossAmountForInnerDeduct(amtT, feeT, minT); v != nil {
			switch v.Code {
			case "MIN_WITHDRAW_AMOUNT_NOT_CONFIGURED":
				return nil, errx.MinWithdrawAmountNotConfigured(assetCode, chainCode)
			case "AMOUNT_MUST_EXCEED_FEE":
				return nil, errx.AmountMustExceedFee(amtT.String(), feeT.String())
			case "AMOUNT_BELOW_MIN_PLUS_FEE":
				meta := v.Meta
				return nil, errx.AmountBelowMinPlusFee(
					meta["amount"],
					meta["fee"],
					meta["min_withdraw"],
					meta["required_amount"],
					meta["net_amount"],
				)
			default:
				return nil, errx.InvalidParam(v.Code)
			}
		}
	}

	return &pb.EstimateWithdrawFeeResp{
		Success:              true,
		Message:              "ok",
		Fee:                  feeStr,
		MinWithdraw:          minWithdrawStr,
		FeeRuleSource:        feeSource, // global|asset
		FeeRuleSummary:       feeRuleSummary,
		MatchedRuleMinAmount: matchedMinAmount,
		MatchedRuleMaxAmount: matchedMaxAmount,
	}, nil
}
