package logic

import (
	"context"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestCalcUsdByRate(t *testing.T) {
	rate := decimal.NewFromInt(1)

	usd, ok := calcUsdByRate("11111", rate)
	require.True(t, ok)
	require.Equal(t, "11111.00", usd)

	_, ok = calcUsdByRate("bad", rate)
	require.False(t, ok)
}

func TestCalcTransferBatchUsdRequiresRate(t *testing.T) {
	usd, ok := calcTransferBatchUsd(context.Background(), nil, "USDT", "1")
	require.False(t, ok)
	require.Equal(t, "", usd)
}
