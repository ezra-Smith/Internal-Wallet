package logic

import (
	"strings"

	"github.com/shopspring/decimal"

	"internalwallet/services/business/rpc/internal/model"
)

func normalizeCode(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}

type effectiveCurrencySettings struct {
	Web3DepositEnabled     bool
	Web3WithdrawEnabled    bool
	UseGlobalWithdrawFee   bool
	UseGlobalWithdrawAudit bool
}

func defaultEffectiveCurrencySettings() effectiveCurrencySettings {
	return effectiveCurrencySettings{
		Web3DepositEnabled:     true,
		Web3WithdrawEnabled:    true,
		UseGlobalWithdrawFee:   true,
		UseGlobalWithdrawAudit: true,
	}
}

func calcWithdrawFee(amountStr string, rules []model.CurrencyWithdrawFeeRuleModel) (decimal.Decimal, bool) {
	amt, err := decimal.NewFromString(strings.TrimSpace(amountStr))
	if err != nil || amt.IsNegative() {
		return decimal.Zero, false
	}
	total := decimal.Zero
	for _, r := range rules {
		// Check if the rule applies to this withdrawal amount (amount range filtering)
		// If min_amount is set, check: amount >= min_amount
		if r.MinAmount != nil && strings.TrimSpace(*r.MinAmount) != "" {
			minAmt, err := decimal.NewFromString(strings.TrimSpace(*r.MinAmount))
			if err == nil && amt.LessThan(minAmt) {
				continue // Skip this rule: amount < min_amount
			}
		}
		// If max_amount is set, check: amount < max_amount
		// If max_amount is NULL or empty, the rule applies to all amounts >= min_amount (no upper limit)
		if r.MaxAmount != nil && strings.TrimSpace(*r.MaxAmount) != "" {
			maxAmt, err := decimal.NewFromString(strings.TrimSpace(*r.MaxAmount))
			if err == nil && !amt.LessThan(maxAmt) {
				continue // Skip this rule: amount >= max_amount
			}
		}

		rt := strings.ToLower(strings.TrimSpace(r.RuleType))
		val, err := decimal.NewFromString(strings.TrimSpace(r.Value))
		if err != nil || val.IsNegative() {
			continue
		}
		switch rt {
		case "fixed":
			total = total.Add(val)
		case "percent":
			part := amt.Mul(val).Div(decimal.NewFromInt(100))
			if r.MinFee != nil {
				if minD, err := decimal.NewFromString(strings.TrimSpace(*r.MinFee)); err == nil && minD.GreaterThan(part) {
					part = minD
				}
			}
			if r.MaxFee != nil {
				if maxD, err := decimal.NewFromString(strings.TrimSpace(*r.MaxFee)); err == nil && part.GreaterThan(maxD) {
					part = maxD
				}
			}
			if part.IsNegative() {
				part = decimal.Zero
			}
			total = total.Add(part)
		}
	}
	if total.IsNegative() {
		total = decimal.Zero
	}
	return total, true
}

func feeTextForRules(assetCode string, rules []model.CurrencyWithdrawFeeRuleModel) string {
	fixedSum := decimal.Zero
	percentSum := decimal.Zero
	hasRules := false
	for _, r := range rules {
		rt := strings.ToLower(strings.TrimSpace(r.RuleType))
		val, err := decimal.NewFromString(strings.TrimSpace(r.Value))
		if err != nil || val.IsNegative() {
			continue
		}
		hasRules = true
		switch rt {
		case "fixed":
			fixedSum = fixedSum.Add(val)
		case "percent":
			percentSum = percentSum.Add(val)
		}
	}
	assetCode = normalizeCode(assetCode)
	parts := make([]string, 0, 2)
	if fixedSum.GreaterThan(decimal.Zero) {
		parts = append(parts, fixedSum.String()+" "+assetCode)
	}
	if percentSum.GreaterThan(decimal.Zero) {
		parts = append(parts, percentSum.String()+"%")
	}
	if len(parts) == 0 {
		if hasRules {
			return "0 " + assetCode
		}
		return ""
	}
	return strings.Join(parts, " + ")
}

// findMatchedRuleRange 找到与给定金额匹配的规则的金额区间
// 返回 (minAmount, maxAmount)，其中 maxAmount 为空字符串表示无上限
func findMatchedRuleRange(amt decimal.Decimal, rules []model.CurrencyWithdrawFeeRuleModel) (string, string) {
	for _, r := range rules {
		// 检查规则是否适用于当前金额
		matchMin := true
		matchMax := true

		// 检查最小金额
		if r.MinAmount != nil && strings.TrimSpace(*r.MinAmount) != "" {
			minAmt, err := decimal.NewFromString(strings.TrimSpace(*r.MinAmount))
			if err == nil && amt.LessThan(minAmt) {
				matchMin = false
			}
		}

		// 检查最大金额
		if r.MaxAmount != nil && strings.TrimSpace(*r.MaxAmount) != "" {
			maxAmt, err := decimal.NewFromString(strings.TrimSpace(*r.MaxAmount))
			if err == nil && !amt.LessThan(maxAmt) {
				matchMax = false
			}
		}

		// 如果当前规则匹配，返回其金额区间
		if matchMin && matchMax {
			minStr := ""
			maxStr := ""
			if r.MinAmount != nil {
				minStr = strings.TrimSpace(*r.MinAmount)
			}
			if r.MaxAmount != nil {
				maxStr = strings.TrimSpace(*r.MaxAmount)
			}
			return minStr, maxStr
		}
	}

	// 没有找到匹配的规则，返回空字符串
	return "", ""
}

// getWithdrawAmountRangeFromRules 从所有提现手续费规则中获取最小值和最大值
// 返回 (minAmount, maxAmount, hasRules)
// - minAmount: 所有规则中最小的 min_amount，如果没有设置返回空字符串
// - maxAmount: 所有规则中最大的 max_amount，如果存在 null（表示无上限）则返回 nil，否则返回最大值
// - hasRules: 是否存在有效规则
func getWithdrawAmountRangeFromRules(rules []model.CurrencyWithdrawFeeRuleModel) (minAmount *string, maxAmount *string, hasRules bool) {
	if len(rules) == 0 {
		return nil, nil, false
	}

	var globalMin *decimal.Decimal
	var globalMax *decimal.Decimal
	hasAnyNullMax := false // 记录是否存在 max_amount 为 null 的规则（表示无上限）

	for _, r := range rules {
		if !r.Enabled {
			continue
		}
		hasRules = true

		// 处理 min_amount：找所有规则中的最小值
		if r.MinAmount != nil && strings.TrimSpace(*r.MinAmount) != "" {
			minVal, err := decimal.NewFromString(strings.TrimSpace(*r.MinAmount))
			if err == nil {
				if globalMin == nil || minVal.LessThan(*globalMin) {
					globalMin = &minVal
				}
			}
		}

		// 处理 max_amount：找所有规则中的最大值
		// 如果存在 max_amount 为 null 或空的规则，说明有无上限的规则
		if r.MaxAmount == nil || strings.TrimSpace(*r.MaxAmount) == "" {
			hasAnyNullMax = true
		} else {
			maxVal, err := decimal.NewFromString(strings.TrimSpace(*r.MaxAmount))
			if err == nil {
				if globalMax == nil || maxVal.GreaterThan(*globalMax) {
					globalMax = &maxVal
				}
			}
		}
	}

	if !hasRules {
		return nil, nil, false
	}

	// 构建返回值
	var minResult *string
	var maxResult *string

	if globalMin != nil {
		minStr := globalMin.String()
		minResult = &minStr
	}

	// 如果存在任何一条规则的 max_amount 为 null，说明无上限，返回 nil
	if hasAnyNullMax {
		maxResult = nil
	} else if globalMax != nil {
		maxStr := globalMax.String()
		maxResult = &maxStr
	}

	return minResult, maxResult, true
}
