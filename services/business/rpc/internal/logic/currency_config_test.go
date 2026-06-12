package logic

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"internalwallet/services/business/rpc/internal/model"
)

func TestCalcWithdrawFee_FixedAndPercent(t *testing.T) {
	min := "1"
	max := "10"
	rules := []model.CurrencyWithdrawFeeRuleModel{
		{RuleType: "fixed", Value: "2"},
		{RuleType: "percent", Value: "1.5", MinFee: &min, MaxFee: &max}, // 1.5% of 100 = 1.5
	}
	fee, ok := calcWithdrawFee("100", rules)
	require.True(t, ok)
	require.True(t, fee.Equal(decimal.RequireFromString("3.5")))
}

func TestCalcWithdrawFee_PercentMinMax(t *testing.T) {
	min := "5"
	max := "6"
	rules := []model.CurrencyWithdrawFeeRuleModel{
		{RuleType: "percent", Value: "1", MinFee: &min, MaxFee: &max}, // 1% of 1000 = 10 -> capped to 6
	}
	fee, ok := calcWithdrawFee("1000", rules)
	require.True(t, ok)
	require.True(t, fee.Equal(decimal.RequireFromString("6")))
}

func TestCalcWithdrawFee_InvalidAmount(t *testing.T) {
	rules := []model.CurrencyWithdrawFeeRuleModel{{RuleType: "fixed", Value: "1"}}
	_, ok := calcWithdrawFee("bad", rules)
	require.False(t, ok)
}
