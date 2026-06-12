package logic

import (
	"context"
	"github.com/shopspring/decimal"
	"strings"

	"internalwallet/services/business/rpc/internal/model"
	"internalwallet/services/business/rpc/internal/svc"
)

// GetEffectiveWithdrawMinAmount 获取有效的提现最小金额
// 综合考虑费率规则中的 MinAmount 和 currency_chain_settings 中的 MinWithdrawAmount，取较大值
//
// 参数:
//   - ctx: 上下文
//   - svcCtx: 服务上下文
//   - assetCode: 资产代码
//   - chainCode: 链代码
//   - useGlobalWithdrawFee: 是否使用全局费率
//   - settingsMinAmount: currency_chain_settings.min_withdraw_amount 的值
//
// 返回:
//   - 有效的最小提现金额字符串（如 "0.01"）
func GetEffectiveWithdrawMinAmount(ctx context.Context, svcCtx *svc.ServiceContext, assetCode, chainCode string, useGlobalWithdrawFee bool, settingsMinAmount *string) string {
	// 从费率规则中获取规则
	rules := loadWithdrawFeeRules(ctx, svcCtx, assetCode, chainCode, useGlobalWithdrawFee)

	// 调用 GetEffectiveWithdrawMinAmountWithRules 进行统一处理（取三个值的最大值）
	return GetEffectiveWithdrawMinAmountWithRules(rules, settingsMinAmount)
}

// GetEffectiveWithdrawMinAmountWithRules 获取有效的提现最小金额（使用已查询的规则）
// 综合考虑费率规则中的 MinAmount 和 currency_chain_settings 中的 MinWithdrawAmount，取较大值
// 此函数接收已查询的规则，避免重复查询数据库
//
// 参数:
//   - rules: 已查询的费率规则
//   - settingsMinAmount: currency_chain_settings.min_withdraw_amount 的值
//
// 返回:
//   - 有效的最小提现金额字符串（如 "0.01"）
func GetEffectiveWithdrawMinAmountWithRules(rules []model.CurrencyWithdrawFeeRuleModel, settingsMinAmount *string) string {
	// 默认最小金额
	defaultMin := "0.01"

	// 从费率规则中获取 MinAmount
	ruleMinAmount := ""
	if len(rules) > 0 {
		// 从规则中提取最小金额
		minAmount, _, _ := getWithdrawAmountRangeFromRules(rules)
		if minAmount != nil && strings.TrimSpace(*minAmount) != "" {
			ruleMinAmount = strings.TrimSpace(*minAmount)
		}
	}

	// 收集所有需要比较的金额值
	candidates := []string{defaultMin}
	if ruleMinAmount != "" {
		candidates = append(candidates, ruleMinAmount)
	}
	if settingsMinAmount != nil && strings.TrimSpace(*settingsMinAmount) != "" {
		candidates = append(candidates, strings.TrimSpace(*settingsMinAmount))
	}

	// 在所有候选值中找出最大值
	finalMinAmount := defaultMin
	maxDec, err := decimal.NewFromString(defaultMin)
	if err != nil {
		return defaultMin
	}

	for _, candidate := range candidates {
		if candDec, err := decimal.NewFromString(candidate); err == nil {
			if candDec.GreaterThan(maxDec) {
				maxDec = candDec
				finalMinAmount = candidate
			}
		}
	}

	return finalMinAmount
}

// GetEffectiveDepositMinAmount 获取有效的充值最小金额
// 优先使用 currency_chain_settings.min_deposit_amount，否则使用默认值
//
// 参数:
//   - settingsMinDepositAmount: currency_chain_settings.min_deposit_amount 的值
//   - assetCode: 资产代码（用于默认值）
//
// 返回:
//   - 有效的最小充值金额字符串（如 "0.01"）
func GetEffectiveDepositMinAmount(settingsMinDepositAmount *string, assetCode string) string {
	if settingsMinDepositAmount != nil && strings.TrimSpace(*settingsMinDepositAmount) != "" {
		return strings.TrimSpace(*settingsMinDepositAmount)
	}
	// 默认充值最小金额
	return "0.01"
}

// loadWithdrawFeeRules 加载提现费率规则（内部使用）
// 优先级：链级别全局配置 > 币种全局配置 > 单独费率规则
func loadWithdrawFeeRules(ctx context.Context, svcCtx *svc.ServiceContext, assetCode, chainCode string, useGlobalWithdrawFee bool) []model.CurrencyWithdrawFeeRuleModel {
	chainCode = normalizeCode(chainCode)
	var rules []model.CurrencyWithdrawFeeRuleModel

	// 步骤1：如果开启了"使用全局"，优先查询链级别的全局配置（如 ETH、BSC、TRON 等）
	if useGlobalWithdrawFee && svcCtx.CurrencyGlobalWithdrawFeeRuleRepository != nil {
		if gr, _ := svcCtx.CurrencyGlobalWithdrawFeeRuleRepository.ListByChain(ctx, chainCode); len(gr) > 0 {
			for _, g := range gr {
				rules = append(rules, model.CurrencyWithdrawFeeRuleModel{
					RuleType:  g.RuleType,
					Value:     g.Value,
					MinFee:    g.MinFee,
					MaxFee:    g.MaxFee,
					MinAmount: g.MinAmount,
					MaxAmount: g.MaxAmount,
					Enabled:   g.Enabled,
					SortOrder: g.SortOrder,
				})
			}
			// 如果找到链级别全局配置，直接返回，不再查币种配置
			return rules
		}
	}

	// 步骤2：如果链级别全局配置不存在，再查询币种的全局费率规则
	if useGlobalWithdrawFee && svcCtx.CurrencyGlobalWithdrawFeeRuleRepository != nil {
		if gr, _ := svcCtx.CurrencyGlobalWithdrawFeeRuleRepository.ListByAsset(ctx, assetCode); len(gr) > 0 {
			for _, g := range gr {
				rules = append(rules, model.CurrencyWithdrawFeeRuleModel{
					RuleType:  g.RuleType,
					Value:     g.Value,
					MinFee:    g.MinFee,
					MaxFee:    g.MaxFee,
					MinAmount: g.MinAmount,
					MaxAmount: g.MaxAmount,
					Enabled:   g.Enabled,
					SortOrder: g.SortOrder,
				})
			}
			return rules
		}
	}

	// 步骤3：如果全局配置都不存在，查询单独的费率规则配置（per-asset-chain）作为回退
	if svcCtx.CurrencyWithdrawFeeRuleRepository != nil {
		if rr, _ := svcCtx.CurrencyWithdrawFeeRuleRepository.ListByAssetChain(ctx, assetCode, chainCode); len(rr) > 0 {
			rules = rr
		}
	}

	return rules
}
