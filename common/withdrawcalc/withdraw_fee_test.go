package withdrawcalc

import (
	"encoding/json"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestBuildFeeRuleSummary_FixedAndPercent(t *testing.T) {
	min := "1"
	max := "10"
	rules := []FeeRuleSnapshotItem{
		{ID: 1, RuleType: "fixed", Value: "2"},
		{ID: 2, RuleType: "percent", Value: "1.5", MinFee: &min, MaxFee: &max},
	}

	fee, summary, b := BuildWithdrawFeeSnapshot(decimal.RequireFromString("100"), "100", "USDT", "TRX", "global", rules)
	require.True(t, fee.GreaterThan(decimal.Zero))
	require.Equal(t, "2 USDT + 1.5% (min 1 max 10)", summary)
	require.NotEmpty(t, b)

	var snap map[string]any
	require.NoError(t, json.Unmarshal(b, &snap))
	require.Equal(t, "global", snap["source"])
}

func TestBuildWithdrawFeeSnapshot_PercentMaxApplied(t *testing.T) {
	min := "5"
	max := "6"
	rules := []FeeRuleSnapshotItem{
		{ID: 100, RuleType: "percent", Value: "1", MinFee: &min, MaxFee: &max},
	}

	amount := decimal.RequireFromString("1000") // 1% -> 10 -> capped to 6
	total, summary, b := BuildWithdrawFeeSnapshot(amount, "1000", "USDT", "TRX", "global", rules)

	require.True(t, total.Equal(decimal.RequireFromString("6")))
	require.Equal(t, "1% (min 5 max 6)", summary)

	var snap struct {
		Source    string `json:"source"`
		AssetCode string `json:"asset_code"`
		ChainCode string `json:"chain_code"`
		Amount    string `json:"amount"`
		Fee       string `json:"fee"`
		Breakdown struct {
			Total string `json:"total"`
			Parts []struct {
				MaxApplied *string `json:"max_fee_applied,omitempty"`
				MinApplied *string `json:"min_fee_applied,omitempty"`
			} `json:"parts"`
		} `json:"breakdown"`
	}
	require.NoError(t, json.Unmarshal(b, &snap))
	require.Equal(t, "global", snap.Source)
	require.Equal(t, "USDT", snap.AssetCode)
	require.Equal(t, "TRX", snap.ChainCode)
	require.Equal(t, "1000", snap.Amount)
	require.Equal(t, "6", snap.Fee)
	require.Equal(t, "6", snap.Breakdown.Total)
	require.Len(t, snap.Breakdown.Parts, 1)
	require.NotNil(t, snap.Breakdown.Parts[0].MaxApplied)
	require.Equal(t, "6", *snap.Breakdown.Parts[0].MaxApplied)
	require.Nil(t, snap.Breakdown.Parts[0].MinApplied)
}

func TestValidateGrossAmountForInnerDeduct(t *testing.T) {
	amt := TruncateAccounting(decimal.RequireFromString("1.2"))
	fee := TruncateAccounting(decimal.RequireFromString("3"))
	minW := TruncateAccounting(decimal.RequireFromString("0.1"))
	v := ValidateGrossAmountForInnerDeduct(amt, fee, minW)
	require.NotNil(t, v)
	require.Equal(t, "AMOUNT_MUST_EXCEED_FEE", v.Code)
}

