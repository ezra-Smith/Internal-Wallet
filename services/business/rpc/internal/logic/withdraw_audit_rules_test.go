package logic

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMatchWithdrawAuditStrategy_MatchesOneRule(t *testing.T) {
	max := "200"
	rules := []withdrawAuditRuleItem{
		{ID: 1, MinAmount: "0", MaxAmount: &max, Strategy: "manual_auto"},
	}

	got, ok, err := matchWithdrawAuditStrategy("100", rules)
	require.NoError(t, err)
	require.True(t, ok)
	require.NotNil(t, got)
	require.Equal(t, int64(1), got.ID)
	require.Equal(t, "manual_auto", got.Strategy)
}

func TestMatchWithdrawAuditStrategy_MaxIsExclusive(t *testing.T) {
	max := "100"
	rules := []withdrawAuditRuleItem{
		{ID: 1, MinAmount: "0", MaxAmount: &max, Strategy: "auto"},
	}

	got, ok, err := matchWithdrawAuditStrategy("100", rules)
	require.NoError(t, err)
	require.False(t, ok)
	require.Nil(t, got)
}

func TestMatchWithdrawAuditStrategy_MultipleMatchesError(t *testing.T) {
	rules := []withdrawAuditRuleItem{
		{ID: 1, MinAmount: "0", Strategy: "auto"},
		{ID: 2, MinAmount: "0", Strategy: "manual_auto"},
	}

	_, ok, err := matchWithdrawAuditStrategy("10", rules)
	require.Error(t, err)
	require.False(t, ok)
}
