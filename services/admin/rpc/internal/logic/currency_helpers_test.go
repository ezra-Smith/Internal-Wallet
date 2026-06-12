package logic

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnforceBaseCurrencySwapEnabled(t *testing.T) {
	require.True(t, enforceBaseCurrencySwapEnabled("USDT", false))
	require.True(t, enforceBaseCurrencySwapEnabled("usdt", false))
	require.True(t, enforceBaseCurrencySwapEnabled("  USDT ", true))

	require.False(t, enforceBaseCurrencySwapEnabled("BTC", false))
	require.True(t, enforceBaseCurrencySwapEnabled("BTC", true))
}

func TestParseNonNegativeDecimal(t *testing.T) {
	_, ok := parseNonNegativeDecimal("1.23")
	require.True(t, ok)
	_, ok = parseNonNegativeDecimal("-1")
	require.False(t, ok)
	_, ok = parseNonNegativeDecimal("bad")
	require.False(t, ok)
}
