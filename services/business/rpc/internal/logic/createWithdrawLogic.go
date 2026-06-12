package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"internalwallet/common/constants"
	"internalwallet/common/middleware"
	"internalwallet/common/utils"
	"internalwallet/common/withdrawcalc"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/model"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type CreateWithdrawLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateWithdrawLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateWithdrawLogic {
	return &CreateWithdrawLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateWithdrawLogic) CreateWithdraw(in *pb.CreateWithdrawReq) (*pb.CreateWithdrawResp, error) {
	if in == nil || in.Asset == "" || in.Chain == "" || in.Amount == "" || in.Address == "" {
		return nil, errx.InvalidParam("invalid params")
	}
	uidStr := middleware.GetUserID(l.ctx)
	if uidStr == "" {
		return nil, errx.Unauthorized("unauthorized")
	}
	id, _ := strconv.ParseInt(uidStr, 10, 64)
	if id <= 0 {
		return nil, errx.Unauthorized("unauthorized")
	}

	// 检查安全冷却期（修改交易密码或绑定GA后24小时内禁止提现）
	cooldownInfo, err := CheckSecurityCooldown(l.ctx, l.svcCtx, id)
	if err != nil {
		l.Logger.Errorf("Failed to check security cooldown for user %d: %v", id, err)
		// 降级处理，不阻塞主流程
	} else if cooldownInfo.InCooldown {
		errMsg := GetCooldownErrorMessage(l.ctx, cooldownInfo.Reason, cooldownInfo.RemainingSeconds)
		l.Logger.Infof("User %d in security cooldown (reason=%s), rejecting withdraw", id, cooldownInfo.Reason)
		return nil, errx.SecurityCooldownActiveWithMessage(errMsg)
	}

	tradePassword := strings.TrimSpace(in.TradePassword)
	hasTrade := tradePassword != ""
	hasBio := in.Biometric != nil
	if hasTrade == hasBio {
		return nil, errx.InvalidParam("trade_password or biometric required")
	}
	if hasTrade {
		// Verify trade password - 带尝试次数限制
		tradePasswordHash, hasTradePassword, err := l.svcCtx.UserAccountRepository.GetTradePasswordHash(l.ctx, id)
		if err != nil {
			l.Logger.Errorf("Failed to get trade password hash: %v", err)
			return nil, errx.Internal("internal error")
		}
		if !hasTradePassword || tradePasswordHash == "" {
			return nil, errx.TradePasswordNotSet()
		}
		if err := VerifyTradePasswordSimple(l.ctx, l.svcCtx, id, tradePassword, tradePasswordHash, "withdraw"); err != nil {
			return nil, err
		}
	} else {
		payloadHash, err := calcWithdrawPayloadHash(in.Asset, in.Chain, in.Amount, in.Address, in.MemoTag)
		if err != nil {
			return nil, errx.InvalidParam("invalid params")
		}
		if err := verifyBiometricProof(l.ctx, l.svcCtx, id, biometricSceneWithdraw, payloadHash, in.Biometric); err != nil {
			return nil, err
		}
	}

	assetCode := normalizeCode(in.Asset)
	chainCode := normalizeCode(in.Chain)

	// Validate address format
	toAddress := strings.TrimSpace(in.Address)
	if !isValidWalletAddress(chainCode, toAddress) {
		return nil, errx.InvalidParam("invalid address format")
	}

	// Block withdraw if destination hits on-chain sensitive address blacklist.
	if res, err := CheckSensitiveAddressBlacklist(l.ctx, l.svcCtx, chainCode, toAddress); err != nil {
		l.Logger.Errorf("check sensitive address blacklist failed: %v", err)
		return nil, errx.RiskCheckFailed("risk check failed")
	} else if res != nil && res.IsBlacklisted {
		return nil, errx.RiskCheckFailed(res.Message)
	}

	// Enforce currency feature toggles (Web3 withdraw) and per-chain enablement.
	if l.svcCtx.AccountingRpc != nil {
		accResp, err := l.svcCtx.AccountingRpc.GetAsset(l.ctx, &pb.GetAssetRequest{Code: assetCode})
		if err != nil || accResp == nil || !accResp.Success || accResp.Item == nil || accResp.Item.Status != 1 {
			return nil, errx.AssetNotAvailable()
		}
	}
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

	amountStr := strings.TrimSpace(in.Amount)
	if amountStr == "" {
		return nil, errx.InvalidParam("AMOUNT_REQUIRED")
	}
	amt, err := decimal.NewFromString(amountStr)
	if err != nil {
		return nil, errx.InvalidParam("INVALID_AMOUNT_FORMAT")
	}
	if !amt.GreaterThan(decimal.Zero) {
		return nil, errx.InvalidParam("AMOUNT_MUST_BE_POSITIVE")
	}

	if l.svcCtx.CurrencyChainSettingsRepository == nil {
		return nil, errx.Internal("currency chain settings not configured")
	}
	chainSetting, csErr := l.svcCtx.CurrencyChainSettingsRepository.FindByAssetChain(l.ctx, assetCode, chainCode)
	if csErr != nil || chainSetting == nil {
		// min_withdraw_amount is mandatory for withdraw creation (config required)
		return nil, errx.MinWithdrawAmountNotConfigured(assetCode, chainCode)
	}
	if chainSetting.Status != 1 || !chainSetting.WithdrawEnabled {
		return nil, errx.NetworkNotAvailable()
	}
	if chainSetting.MinWithdrawAmount == nil || strings.TrimSpace(*chainSetting.MinWithdrawAmount) == "" {
		return nil, errx.MinWithdrawAmountNotConfigured(assetCode, chainCode)
	}
	minWithdraw, minErr := decimal.NewFromString(strings.TrimSpace(*chainSetting.MinWithdrawAmount))
	if minErr != nil || minWithdraw.LessThanOrEqual(decimal.Zero) {
		return nil, errx.MinWithdrawAmountNotConfigured(assetCode, chainCode)
	}

	// Calculate withdraw fee based on rules and persist a snapshot (for audit transparency).
	feeSource, feeRuleItems := LoadWithdrawFeeRuleItems(l.ctx, l.svcCtx, assetCode, chainCode, eff.UseGlobalWithdrawFee)
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
		case "MIN_WITHDRAW_AMOUNT_NOT_CONFIGURED":
			return nil, errx.MinWithdrawAmountNotConfigured(assetCode, chainCode)
		case "AMOUNT_MUST_EXCEED_FEE":
			return nil, errx.AmountMustExceedFee(amountStrTruncated, feeStrTruncated)
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

	// Decide withdraw audit strategy based on rules.
	// Whitelist (per address+token+limit): bypass audit -> force auto.
	var strategy string
	var ok bool
	var auditSnapshot []byte
	auditEvents := make([]struct {
		eventType string
		summary   string
		details   any
	}, 0, 3)
	if l.svcCtx.UserWithdrawAuditWhitelistRuleRepository != nil {
		addr := normalizeWhitelistAddress(toAddress)
		if addr != "" {
			rule, rErr := l.svcCtx.UserWithdrawAuditWhitelistRuleRepository.GetEnabledMatch(l.ctx, id, assetCode, chainCode, addr)
			if rErr != nil {
				auditEvents = append(auditEvents, struct {
					eventType string
					summary   string
					details   any
				}{eventType: constants.WithdrawEventAuditWhitelistMiss, summary: "Withdrawal audit whitelist miss", details: map[string]any{
					"address": addr,
					"reason":  "query failed",
					"error":   strings.TrimSpace(rErr.Error()),
				}})
			} else if rule == nil {
				auditEvents = append(auditEvents, struct {
					eventType string
					summary   string
					details   any
				}{eventType: constants.WithdrawEventAuditWhitelistMiss, summary: "Withdrawal audit whitelist miss", details: map[string]any{
					"address": addr,
					"reason":  "no rule matched",
				}})
			} else {
				limit, pErr := decimal.NewFromString(strings.TrimSpace(rule.LimitUSDT))
				if pErr != nil || limit.IsNegative() {
					auditEvents = append(auditEvents, struct {
						eventType string
						summary   string
						details   any
					}{eventType: constants.WithdrawEventAuditWhitelistMiss, summary: "Withdrawal audit whitelist miss", details: map[string]any{
						"rule_id":    rule.ID,
						"address":    addr,
						"limit_usdt": strings.TrimSpace(rule.LimitUSDT),
						"reason":     "invalid limit_usdt",
					}})
				} else {
					hit := false
					var amountUSDTStr string
					if limit.Equal(decimal.Zero) {
						hit = true
						amountUSDTStr = "0"
					} else if price, okPrice := getUSDTPriceFromRedis(l.ctx, l.svcCtx.RedisClient, assetCode); okPrice {
						amountUSDT := amt.Mul(price).RoundCeil(8)
						amountUSDTStr = amountUSDT.String()
						if amountUSDT.LessThanOrEqual(limit) {
							hit = true
						}
					}

					if hit {
						strategy = "auto"
						ok = true
						ruleSource := strings.TrimSpace(rule.Source)
						if ruleSource == "" {
							ruleSource = "admin"
						}
						snap := withdrawAuditRuleSnapshot{
							MatchedBy: "whitelist",
							Source:    ruleSource,
							RuleID:    rule.ID,
							AssetCode: assetCode,
							ChainCode: chainCode,
							Strategy:  "auto",
							UseGlobal: eff.UseGlobalWithdrawAudit,
							Amount:    amountStr,
						}
						auditSnapshot, _ = json.Marshal(snap)
						auditEvents = append(auditEvents, struct {
							eventType string
							summary   string
							details   any
						}{eventType: constants.WithdrawEventAuditWhitelistHit, summary: "Withdrawal audit whitelist hit", details: map[string]any{
							"rule_id":        rule.ID,
							"address":        addr,
							"rule_source":    ruleSource,
							"limit_usdt":     strings.TrimSpace(rule.LimitUSDT),
							"amount_usdt":    amountUSDTStr,
							"matched_by":     "user_withdraw_audit_whitelist_rule",
							"audit_snapshot": json.RawMessage(auditSnapshot),
						}})
					} else {
						reason := "exceeds whitelist limit"
						if limit.Equal(decimal.Zero) {
							reason = "unexpected"
						} else if amountUSDTStr == "" {
							reason = "price not available"
						}
						auditEvents = append(auditEvents, struct {
							eventType string
							summary   string
							details   any
						}{eventType: constants.WithdrawEventAuditWhitelistMiss, summary: "Withdrawal audit whitelist miss", details: map[string]any{
							"rule_id":     rule.ID,
							"address":     addr,
							"limit_usdt":  strings.TrimSpace(rule.LimitUSDT),
							"amount_usdt": amountUSDTStr,
							"reason":      reason,
						}})
					}
				}
			}
		}
	}

	// Legacy global whitelist: bypass audit -> force auto.
	if !ok && l.svcCtx.UserWhitelistSettingsRepository != nil {
		if s, _ := l.svcCtx.UserWhitelistSettingsRepository.GetByUserID(l.ctx, id); s != nil && s.BypassWithdrawAudit {
			strategy = "auto"
			ok = true
			snap := withdrawAuditRuleSnapshot{
				MatchedBy: "legacy",
				Source:    "legacy",
				Strategy:  "auto",
				UseGlobal: eff.UseGlobalWithdrawAudit,
				Amount:    amountStr,
			}
			auditSnapshot, _ = json.Marshal(snap)
			auditEvents = append(auditEvents, struct {
				eventType string
				summary   string
				details   any
			}{eventType: constants.WithdrawEventAuditWhitelistHit, summary: "Withdrawal audit whitelist hit", details: map[string]any{
				"matched_by":     "user_whitelist_settings",
				"bypass_audit":   true,
				"audit_snapshot": json.RawMessage(auditSnapshot),
			}})
		}
	}
	if !ok {
		var err error
		var snap []byte
		strategy, ok, snap, err = pickWithdrawAuditStrategy(l.ctx, l.svcCtx, assetCode, chainCode, amountStr, eff.UseGlobalWithdrawAudit)
		if err != nil {
			l.Logger.Errorf("pick withdraw audit strategy failed: %v", err)
			return nil, errx.Internal("withdraw audit config error")
		}
		// 如果策略未匹配，表示金额不在任何规则范围内，拒绝提现
		if !ok {
			l.Logger.Errorf("Withdraw rejected: amount %s not in any audit rule range for asset %s chain %s", amountStr, assetCode, chainCode)
			return nil, errx.WithdrawAmountOutOfRange()
		}
		auditSnapshot = snap
		auditEvents = append(auditEvents, struct {
			eventType string
			summary   string
			details   any
		}{eventType: constants.WithdrawEventAuditRuleMatched, summary: "Withdrawal audit rule matched", details: map[string]any{
			"audit_snapshot": json.RawMessage(auditSnapshot),
		}})
	}

	// Auto payout requires ChainRpc; fail early to avoid freezing funds that can't be sent.
	if strategy == "auto" && l.svcCtx.ChainRpc == nil {
		return nil, errx.ServiceNotAvailable("chain rpc")
	}

	var fromAddress string
	if strategy == "auto" {
		addr, err := resolveCompanyHotWalletAddress(l.ctx, l.svcCtx, chainCode)
		if err != nil {
			l.Logger.Errorf("resolve company hot wallet address failed: chain=%s, err=%v", chainCode, err)
			return nil, errx.ServiceNotAvailable("payout wallet")
		}
		fromAddress = addr
	}

	// 检查热钱包余额和 gas 是否足够（提前告知用户）
	checker := NewHotWalletBalanceChecker(l.ctx, l.svcCtx)
	if balanceCheckErr := checker.CheckHotWalletBalance(assetCode, chainCode, amountStrTruncated); balanceCheckErr != nil {
		l.Logger.Infof("热钱包余额不足，拒绝提现申请: user_id=%d, asset=%s, chain=%s, amount=%s, error=%v",
			id, assetCode, chainCode, amountStrTruncated, balanceCheckErr)
		return nil, balanceCheckErr
	}

	if l.svcCtx.DB == nil || l.svcCtx.CurrencyWithdrawOrderRepository == nil {
		return nil, errx.ServiceNotAvailable("db")
	}

	if l.svcCtx.AccountingRpc == nil {
		return nil, errx.ServiceNotAvailable("accounting rpc")
	}

	withdrawID := utils.GenerateID()
	bizRef := strconv.FormatInt(withdrawID, 10)

	var memoTagPtr *string
	if strings.TrimSpace(in.MemoTag) != "" {
		v := strings.TrimSpace(in.MemoTag)
		memoTagPtr = &v
	}

	order := &model.CurrencyWithdrawOrderModel{
		UserID:          id,
		AssetCode:       assetCode,
		ChainCode:       chainCode,
		Amount:          amountStrTruncated,
		Fee:             feeStrTruncated,
		FeeRuleSnapshot: feeSnapshot,
		FeeRuleSummary: func() *string {
			s := strings.TrimSpace(feeRuleSummary)
			if s == "" {
				return nil
			}
			return &s
		}(),
		FeeRuleSource:     strings.TrimSpace(feeSource),
		AuditRuleSnapshot: auditSnapshot,
		FromAddress:       fromAddress,
		ToAddress:         toAddress,
		MemoTag:           memoTagPtr,
		Strategy:          strategy,
		Status: func() string {
			if strategy == "auto" {
				return "processing"
			}
			return "pending"
		}(),
		UpdatedBy: 0,
	}
	order.ID = withdrawID

	// 内扣模式：只冻结 amount（手续费从提现金额中扣除，不额外冻结）
	// 用户输入的 amount 就是从账户扣除的总额，实际到账 = amount - fee
	freezeReq := &pb.FreezeWithdrawRequest{
		IdempotencyKey:   fmt.Sprintf("withdraw:freeze:%d", withdrawID),
		BizRef:           bizRef,
		UserId:           id,
		AssetCode:        assetCode,
		AmountDecimal:    amountStrTruncated, // 只冻结 amount，不加 fee
		ChainCode:        chainCode,
		ToAddress:        toAddress,
		PrincipalDecimal: amountStrTruncated,
		FeeDecimal:       feeStrTruncated,
		Memo:             strings.TrimSpace(in.MemoTag),
	}
	freezeResp, err := callAccountingTxWithRetry(l.ctx, 3, func(ctx context.Context) (*pb.LedgerTxResponse, error) {
		return l.svcCtx.AccountingRpc.FreezeWithdraw(ctx, freezeReq)
	})
	if err != nil || freezeResp == nil || !freezeResp.Success {
		msg := "freeze failed"
		if freezeResp != nil && strings.TrimSpace(freezeResp.Message) != "" {
			msg = strings.TrimSpace(freezeResp.Message)
		}
		return nil, errx.Internal(msg)
	}

	events := make([]*model.CurrencyWithdrawOrderEventModel, 0, 8)
	events = append(events, newWithdrawOrderEvent(withdrawID, constants.WithdrawEventOrderCreated, constants.WithdrawActorUser, id, nil, "Withdrawal order created", map[string]any{
		"user_id":    id,
		"asset_code": assetCode,
		"chain_code": chainCode,
		"amount":     amountStrTruncated,
		"fee":        feeStrTruncated,
		"strategy":   strategy,
		"to_address": toAddress,
		"memo_tag":   strings.TrimSpace(in.MemoTag),
		"created_by": "user",
	}))
	events = append(events, newWithdrawOrderEvent(withdrawID, constants.WithdrawEventFeeCalculated, constants.WithdrawActorSystem, 0, nil, "Withdrawal fee calculated", map[string]any{
		"fee_rule_source":  feeSource,
		"fee_rule_summary": feeRuleSummary,
		"snapshot":         json.RawMessage(feeSnapshot),
	}))
	for _, ae := range auditEvents {
		events = append(events, newWithdrawOrderEvent(withdrawID, ae.eventType, constants.WithdrawActorSystem, 0, nil, ae.summary, ae.details))
	}
	events = append(events, newWithdrawOrderEvent(withdrawID, constants.WithdrawEventAssetFrozen, constants.WithdrawActorSystem, 0, nil, "Asset frozen for withdrawal", map[string]any{
		"tx_id":           freezeResp.TxId,
		"duplicate":       freezeResp.Duplicate,
		"asset_code":      assetCode,
		"amount_decimal":  amountStrTruncated, // 内扣模式：冻结金额=提现金额
		"idempotency_key": freezeReq.IdempotencyKey,
		"biz_ref":         bizRef,
	}))

	if err := l.svcCtx.DB.WithContext(l.ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.WithContext(l.ctx).Create(order).Error; err != nil {
			return err
		}
		if l.svcCtx.CurrencyWithdrawOrderEventRepository != nil && len(events) > 0 {
			if err := l.svcCtx.CurrencyWithdrawOrderEventRepository.WithTx(tx).CreateMultiple(l.ctx, events); err != nil {
				return err
			}
		}
		if strategy == "auto" {
			if l.svcCtx.CurrencyWithdrawPayoutTaskRepository == nil {
				return fmt.Errorf("withdraw payout task repo not configured")
			}
			if err := l.svcCtx.CurrencyWithdrawPayoutTaskRepository.WithTx(tx).CreateIfNotExists(l.ctx, &model.CurrencyWithdrawPayoutTaskModel{
				WithdrawOrderID:      withdrawID,
				Mode:                 constants.WithdrawPayoutModeSystem,
				State:                constants.WithdrawPayoutStatePendingBroadcast,
				MaxBroadcastAttempts: 3,
				NextRetryTime:        time.Now(),
			}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		// Best-effort rollback freeze if DB write fails.
		unfreezeReq := &pb.UnfreezeWithdrawRequest{
			IdempotencyKey: fmt.Sprintf("withdraw:unfreeze:%d", withdrawID),
			BizRef:         bizRef,
			UserId:         id,
			AssetCode:      assetCode,
			AmountDecimal:  amountStrTruncated, // 内扣模式：解冻金额=冻结金额=提现金额
			FinalStatus:    "failed",
			Reason:         "create withdraw order failed",
		}
		_, _ = callAccountingTxWithRetry(l.ctx, 3, func(ctx context.Context) (*pb.LedgerTxResponse, error) {
			return l.svcCtx.AccountingRpc.UnfreezeWithdraw(ctx, unfreezeReq)
		})
		return nil, errx.DBError()
	}

	// Manual strategies create an order for admin review/payout.
	if strategy == "manual_auto" || strategy == "manual_manual" {
		return &pb.CreateWithdrawResp{
			Success:    true,
			Message:    "ok",
			WithdrawId: strconv.FormatInt(withdrawID, 10),
			Fee:        feeStrTruncated,
			Network:    chainCode,
		}, nil
	}

	// Auto: enqueue payout task; worker will broadcast, confirm on-chain, then settle and mark completed.
	return &pb.CreateWithdrawResp{
		Success:    true,
		Message:    "ok",
		WithdrawId: strconv.FormatInt(withdrawID, 10),
		Fee:        feeStrTruncated,
		Network:    chainCode,
	}, nil
}

func normalizeWhitelistAddress(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ""
	}
	if len(addr) == 42 && (strings.HasPrefix(addr, "0x") || strings.HasPrefix(addr, "0X")) {
		hexPart := addr[2:]
		for _, c := range hexPart {
			isHex := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
			if !isHex {
				return addr
			}
		}
		return strings.ToLower(addr)
	}
	return addr
}

type miniTickerValue struct {
	Price string `json:"price"`
	Ts    int64  `json:"ts"`
}

func getUSDTPriceFromRedis(ctx context.Context, rdb *redis.Client, assetCode string) (decimal.Decimal, bool) {
	assetCode = strings.ToUpper(strings.TrimSpace(assetCode))
	if assetCode == "" {
		return decimal.Zero, false
	}
	if assetCode == "USDT" {
		return decimal.NewFromInt(1), true
	}
	if rdb == nil {
		return decimal.Zero, false
	}

	field := assetCode + "USDT"
	val, err := rdb.HGet(ctx, "binance:tickers", field).Result()
	if err != nil {
		return decimal.Zero, false
	}
	var tv miniTickerValue
	if err := json.Unmarshal([]byte(val), &tv); err != nil {
		return decimal.Zero, false
	}
	price, err := decimal.NewFromString(strings.TrimSpace(tv.Price))
	if err != nil || !price.GreaterThan(decimal.Zero) {
		return decimal.Zero, false
	}
	return price, true
}

func mapChainType(code string) pb.ChainRpcType {
	switch code {
	case "TRN", "TRON", "TRX":
		return pb.ChainRpcType_CHAIN_TYPE_TRON
	case "BSC", "BNB":
		return pb.ChainRpcType_CHAIN_TYPE_BSC
	case "ETH", "ETHEREUM":
		return pb.ChainRpcType_CHAIN_TYPE_ETHEREUM
	case "POLYGON", "MATIC":
		return pb.ChainRpcType_CHAIN_TYPE_POLYGON
	default:
		return pb.ChainRpcType_CHAIN_TYPE_UNSPECIFIED
	}
}

func resolveCompanyHotWalletAddress(ctx context.Context, svcCtx *svc.ServiceContext, chainCode string) (string, error) {
	if svcCtx == nil || svcCtx.SignerRpc == nil {
		return "", fmt.Errorf("signer rpc not configured")
	}
	chainCode = strings.ToUpper(strings.TrimSpace(chainCode))
	if chainCode == "" {
		return "", fmt.Errorf("empty chain code")
	}
	resp, err := svcCtx.SignerRpc.GetCompanyWallet(ctx, &pb.GetCompanyWalletRequest{
		Chain:       chainCode,
		AddressType: "hot_primary",
		Temperature: 1, // hot wallet
	})
	if err != nil {
		return "", err
	}
	if resp == nil {
		return "", fmt.Errorf("empty signer response")
	}
	if resp.Code != 0 {
		return "", fmt.Errorf("signer error: code=%d, message=%s", resp.Code, strings.TrimSpace(resp.Message))
	}
	if resp.Wallet == nil || strings.TrimSpace(resp.Wallet.Address) == "" {
		return "", fmt.Errorf("empty company hot wallet address")
	}
	if resp.Wallet.Status != 1 {
		return "", fmt.Errorf("company hot wallet disabled")
	}
	return strings.TrimSpace(resp.Wallet.Address), nil
}
