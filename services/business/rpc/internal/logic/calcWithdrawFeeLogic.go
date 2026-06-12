package logic

import (
	"context"
	"strconv"
	"strings"

	"internalwallet/common/middleware"
	"internalwallet/common/withdrawcalc"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
)

// CalcWithdrawFeeLogic Web2提币手续费计算（校验余额；不冻结资金、不创建订单）
type CalcWithdrawFeeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCalcWithdrawFeeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CalcWithdrawFeeLogic {
	return &CalcWithdrawFeeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CalcWithdrawFeeLogic) CalcWithdrawFee(in *pb.CalcWithdrawFeeReq) (*pb.CalcWithdrawFeeResp, error) {
	if in == nil || strings.TrimSpace(in.Asset) == "" || strings.TrimSpace(in.Chain) == "" || strings.TrimSpace(in.Amount) == "" {
		return nil, errx.InvalidParam("invalid params")
	}

	uidStr := middleware.GetUserID(l.ctx)
	if strings.TrimSpace(uidStr) == "" {
		return nil, errx.Unauthorized("unauthorized")
	}
	uid, _ := strconv.ParseInt(strings.TrimSpace(uidStr), 10, 64)
	if uid <= 0 {
		return nil, errx.Unauthorized("unauthorized")
	}

	assetCode := normalizeCode(in.Asset)
	chainCode := normalizeCode(in.Chain)
	amountStr := strings.TrimSpace(in.Amount)

	amt, err := decimal.NewFromString(amountStr)
	if err != nil {
		return nil, errx.InvalidParam("INVALID_AMOUNT_FORMAT")
	}
	if !amt.GreaterThan(decimal.Zero) {
		return nil, errx.InvalidParam("AMOUNT_MUST_BE_POSITIVE")
	}

	// 1) 判断币是否存在/可用（Accounting 是资产的真源）
	if l.svcCtx.AccountingRpc == nil {
		return nil, errx.ServiceNotAvailable("accounting")
	}
	accAsset, err := l.svcCtx.AccountingRpc.GetAsset(l.ctx, &pb.GetAssetRequest{Code: assetCode})
	if err != nil || accAsset == nil || !accAsset.Success || accAsset.Item == nil || accAsset.Item.Status != 1 {
		return nil, errx.AssetNotAvailable()
	}

	// 2) 判断币种提现开关/费率来源配置
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

	// 3) 判断链是否存在/可提现，且 min_withdraw_amount 已配置
	if l.svcCtx.CurrencyChainSettingsRepository == nil {
		return nil, errx.Internal("currency chain settings not configured")
	}
	cs, err := l.svcCtx.CurrencyChainSettingsRepository.FindByAssetChain(l.ctx, assetCode, chainCode)
	if err != nil || cs == nil {
		return nil, errx.NetworkNotAvailable()
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

	// 4) 计算手续费（复用后台规则）
	feeSource, ruleItems := LoadWithdrawFeeRuleItems(l.ctx, l.svcCtx, assetCode, chainCode, eff.UseGlobalWithdrawFee)
	feeDec, _, _ := withdrawcalc.BuildWithdrawFeeSnapshot(amt, amt.String(), assetCode, chainCode, feeSource, ruleItems)

	amtT := withdrawcalc.TruncateAccounting(amt)
	feeT := withdrawcalc.TruncateAccounting(feeDec)
	minT := withdrawcalc.TruncateAccounting(minWithdrawDec)
	requiredT := withdrawcalc.TruncateAccounting(minT.Add(feeT))

	// 5) 判断余额是否足够（可用余额 >= 提现金额，gross/内扣口径）
	accBal, err := l.svcCtx.AccountingRpc.GetUserBalances(l.ctx, &pb.GetUserBalancesRequest{UserId: uid})
	if err != nil {
		l.Logger.Errorf("GetUserBalances failed: uid=%d err=%v", uid, err)
		return nil, errx.ServiceNotAvailable("accounting")
	}
	available := decimal.Zero
	if accBal != nil {
		for _, it := range accBal.GetItems() {
			if it == nil {
				continue
			}
			if normalizeCode(it.AssetCode) != assetCode {
				continue
			}
			available, _ = decimal.NewFromString(strings.TrimSpace(it.Available))
			break
		}
	}
	if available.LessThan(amtT) {
		return nil, errx.InsufficientBalance()
	}

	return &pb.CalcWithdrawFeeResp{
		Success:           true,
		Message:           "ok",
		Fee:               feeT.String(),
		MinWithdrawAmount: minT.String(),
		RequiredAmount:    requiredT.String(),
	}, nil
}
