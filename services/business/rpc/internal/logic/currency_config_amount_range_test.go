package logic

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"internalwallet/services/business/rpc/internal/model"
)

// TestCalcWithdrawFee_AmountRangeFiltering 测试金额区间筛选功能
func TestCalcWithdrawFee_AmountRangeFiltering(t *testing.T) {
	minAmt1 := "0"
	maxAmt1 := "100"
	minAmt2 := "100"
	maxAmt2 := "10000"

	rules := []model.CurrencyWithdrawFeeRuleModel{
		// 规则1：固定 0.9，适用于 0-100 区间
		{
			RuleType:  "fixed",
			Value:     "0.9",
			MinAmount: &minAmt1,
			MaxAmount: &maxAmt1,
		},
		// 规则2：百分比 1%，适用于 100-10000 区间
		{
			RuleType:  "percent",
			Value:     "1",
			MinAmount: &minAmt2,
			MaxAmount: &maxAmt2,
		},
	}

	// 测试用例1：提现 10 USDT，应该只应用规则1
	fee1, ok := calcWithdrawFee("10", rules)
	require.True(t, ok)
	require.True(t, fee1.Equal(decimal.RequireFromString("0.9")), "Expected 0.9, got %s", fee1.String())

	// 测试用例2：提现 50 USDT，应该只应用规则1
	fee2, ok := calcWithdrawFee("50", rules)
	require.True(t, ok)
	require.True(t, fee2.Equal(decimal.RequireFromString("0.9")), "Expected 0.9, got %s", fee2.String())

	// 测试用例3：提现 100 USDT，应该只应用规则2（边界值，100 >= 100）
	fee3, ok := calcWithdrawFee("100", rules)
	require.True(t, ok)
	require.True(t, fee3.Equal(decimal.RequireFromString("1")), "Expected 1, got %s", fee3.String())

	// 测试用例4：提现 500 USDT，应该只应用规则2
	fee4, ok := calcWithdrawFee("500", rules)
	require.True(t, ok)
	require.True(t, fee4.Equal(decimal.RequireFromString("5")), "Expected 5, got %s", fee4.String())

	// 测试用例5：提现 5000 USDT，应该只应用规则2
	fee5, ok := calcWithdrawFee("5000", rules)
	require.True(t, ok)
	require.True(t, fee5.Equal(decimal.RequireFromString("50")), "Expected 50, got %s", fee5.String())

	// 测试用例6：提现 10000 USDT，不应用任何规则（边界值，10000 >= 10000）
	fee6, ok := calcWithdrawFee("10000", rules)
	require.True(t, ok)
	require.True(t, fee6.Equal(decimal.RequireFromString("0")), "Expected 0, got %s", fee6.String())

	// 测试用例7：提现 15000 USDT，不应用任何规则
	fee7, ok := calcWithdrawFee("15000", rules)
	require.True(t, ok)
	require.True(t, fee7.Equal(decimal.RequireFromString("0")), "Expected 0, got %s", fee7.String())
}

// TestCalcWithdrawFee_NoUpperLimit 测试没有上限的兜底规则
func TestCalcWithdrawFee_NoUpperLimit(t *testing.T) {
	minAmt1 := "0"
	maxAmt1 := "100"
	minAmt2 := "100"
	// 注意：规则2 没有设置 max_amount，表示无上限

	rules := []model.CurrencyWithdrawFeeRuleModel{
		// 规则1：固定 0.9，适用于 0-100 区间
		{
			RuleType:  "fixed",
			Value:     "0.9",
			MinAmount: &minAmt1,
			MaxAmount: &maxAmt1,
		},
		// 规则2：百分比 1%，适用于 100 以上所有金额（无上限）
		{
			RuleType:  "percent",
			Value:     "1",
			MinFee:    strPtr("1"),
			MaxFee:    strPtr("100"),
			MinAmount: &minAmt2,
			MaxAmount: nil, // 无上限，作为兜底规则
		},
	}

	// 测试用例1：提现 10 USDT，应用规则1
	fee1, ok := calcWithdrawFee("10", rules)
	require.True(t, ok)
	require.True(t, fee1.Equal(decimal.RequireFromString("0.9")), "Expected 0.9, got %s", fee1.String())

	// 测试用例2：提现 100 USDT，应用规则2
	fee2, ok := calcWithdrawFee("100", rules)
	require.True(t, ok)
	require.True(t, fee2.Equal(decimal.RequireFromString("1")), "Expected 1, got %s", fee2.String())

	// 测试用例3：提现 500 USDT，应用规则2
	fee3, ok := calcWithdrawFee("500", rules)
	require.True(t, ok)
	require.True(t, fee3.Equal(decimal.RequireFromString("5")), "Expected 5, got %s", fee3.String())

	// 测试用例4：提现 10000 USDT，应用规则2（兜底）
	fee4, ok := calcWithdrawFee("10000", rules)
	require.True(t, ok)
	// 10000 * 1% = 100，但 max_fee = 100，所以是 100
	require.True(t, fee4.Equal(decimal.RequireFromString("100")), "Expected 100, got %s", fee4.String())

	// 测试用例5：提现 50000 USDT，应用规则2（兜底，max_fee 限制）
	fee5, ok := calcWithdrawFee("50000", rules)
	require.True(t, ok)
	// 50000 * 1% = 500，但 max_fee = 100，所以是 100
	require.True(t, fee5.Equal(decimal.RequireFromString("100")), "Expected 100 (capped by max_fee), got %s", fee5.String())
}

func strPtr(s string) *string {
	return &s
}

// TestCalcWithdrawFee_AmountRangeWithMinMaxFee 测试金额区间 + 手续费限制的组合功能
func TestCalcWithdrawFee_AmountRangeWithMinMaxFee(t *testing.T) {
	minAmt := "100"
	maxAmt := "10000"
	minFee := "1"
	maxFee := "100"

	rules := []model.CurrencyWithdrawFeeRuleModel{
		// 百分比 1%，适用于 100-10000 区间，手续费限制为 1-100
		{
			RuleType:  "percent",
			Value:     "1",
			MinAmount: &minAmt,
			MaxAmount: &maxAmt,
			MinFee:    &minFee,
			MaxFee:    &maxFee,
		},
	}

	// 测试用例1：提现 50 USDT，不在区间内，应该不收费
	fee1, ok := calcWithdrawFee("50", rules)
	require.True(t, ok)
	require.True(t, fee1.Equal(decimal.Zero), "Expected 0, got %s", fee1.String())

	// 测试用例2：提现 100 USDT，1% = 1，正好等于 min_fee
	fee2, ok := calcWithdrawFee("100", rules)
	require.True(t, ok)
	require.True(t, fee2.Equal(decimal.RequireFromString("1")), "Expected 1, got %s", fee2.String())

	// 测试用例3：提现 50 USDT（假设在区间内），1% = 0.5，应该使用 min_fee = 1
	minAmt3 := "0"
	rules3 := []model.CurrencyWithdrawFeeRuleModel{
		{
			RuleType:  "percent",
			Value:     "1",
			MinAmount: &minAmt3,
			MaxAmount: &maxAmt,
			MinFee:    &minFee,
			MaxFee:    &maxFee,
		},
	}
	fee3, ok := calcWithdrawFee("50", rules3)
	require.True(t, ok)
	require.True(t, fee3.Equal(decimal.RequireFromString("1")), "Expected 1 (min_fee), got %s", fee3.String())

	// 测试用例4：提现 20000 USDT（假设在区间内），1% = 200，应该使用 max_fee = 100
	maxAmt4 := "50000"
	rules4 := []model.CurrencyWithdrawFeeRuleModel{
		{
			RuleType:  "percent",
			Value:     "1",
			MinAmount: &minAmt,
			MaxAmount: &maxAmt4,
			MinFee:    &minFee,
			MaxFee:    &maxFee,
		},
	}
	fee4, ok := calcWithdrawFee("20000", rules4)
	require.True(t, ok)
	require.True(t, fee4.Equal(decimal.RequireFromString("100")), "Expected 100 (max_fee), got %s", fee4.String())
}

// TestCalcWithdrawFee_NoAmountRange 测试没有金额区间限制的规则（兼容旧数据）
func TestCalcWithdrawFee_NoAmountRange(t *testing.T) {
	rules := []model.CurrencyWithdrawFeeRuleModel{
		// 没有设置 MinAmount/MaxAmount，应该对所有金额生效
		{RuleType: "fixed", Value: "1"},
		{RuleType: "percent", Value: "0.5"},
	}

	// 测试用例1：提现 100 USDT
	fee1, ok := calcWithdrawFee("100", rules)
	require.True(t, ok)
	// 固定 1 + 百分比 0.5% = 1 + 0.5 = 1.5
	require.True(t, fee1.Equal(decimal.RequireFromString("1.5")), "Expected 1.5, got %s", fee1.String())

	// 测试用例2：提现 1000 USDT
	fee2, ok := calcWithdrawFee("1000", rules)
	require.True(t, ok)
	// 固定 1 + 百分比 0.5% = 1 + 5 = 6
	require.True(t, fee2.Equal(decimal.RequireFromString("6")), "Expected 6, got %s", fee2.String())
}

// TestCalcWithdrawFee_OverlappingRanges 测试重叠区间（不推荐，但应该正确处理）
func TestCalcWithdrawFee_OverlappingRanges(t *testing.T) {
	minAmt1 := "0"
	maxAmt1 := "200"
	minAmt2 := "100"
	maxAmt2 := "500"

	rules := []model.CurrencyWithdrawFeeRuleModel{
		// 规则1：固定 1，适用于 0-200
		{RuleType: "fixed", Value: "1", MinAmount: &minAmt1, MaxAmount: &maxAmt1},
		// 规则2：固定 2，适用于 100-500
		{RuleType: "fixed", Value: "2", MinAmount: &minAmt2, MaxAmount: &maxAmt2},
	}

	// 测试用例1：提现 50 USDT，只应用规则1
	fee1, ok := calcWithdrawFee("50", rules)
	require.True(t, ok)
	require.True(t, fee1.Equal(decimal.RequireFromString("1")), "Expected 1, got %s", fee1.String())

	// 测试用例2：提现 150 USDT，在重叠区间，两条规则都应用
	fee2, ok := calcWithdrawFee("150", rules)
	require.True(t, ok)
	require.True(t, fee2.Equal(decimal.RequireFromString("3")), "Expected 3, got %s", fee2.String())

	// 测试用例3：提现 300 USDT，只应用规则2
	fee3, ok := calcWithdrawFee("300", rules)
	require.True(t, ok)
	require.True(t, fee3.Equal(decimal.RequireFromString("2")), "Expected 2, got %s", fee3.String())
}
