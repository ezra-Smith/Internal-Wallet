package oneinch

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdaptQuote(t *testing.T) {
	t.Parallel()

	var dto quoteResponseDTO
	err := json.Unmarshal([]byte(`{
	  "fromToken": {"address":"0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE","symbol":"ETH","name":"Ether","decimals":18,"logoURI":"https://example/eth.png"},
	  "toToken": {"address":"0xA0b86991c6218b36c1d19d4a2e9eb0ce3606eb48","symbol":"USDC","name":"USD Coin","decimals":6,"logoURI":"https://example/usdc.png"},
	  "fromAmount":"1000000000000000000",
	  "toAmount":"2500000000",
	  "estimatedGas":21000
	}`), &dto)
	require.NoError(t, err)

	out, err := AdaptQuote(1, &dto)
	require.NoError(t, err)
	require.Equal(t, int64(1), out.ChainID)
	require.Equal(t, "1inch", out.Provider)
	require.Equal(t, "ETH", out.FromToken.Symbol)
	require.Equal(t, "USDC", out.ToToken.Symbol)
	require.Equal(t, "1000000000000000000", out.FromAmount)
	require.Equal(t, "2500000000", out.ToAmount)
	require.Equal(t, uint64(21000), out.EstimatedGas)
}

func TestAdaptSwap_GasFieldsSupportStringOrNumber(t *testing.T) {
	t.Parallel()

	var dto swapResponseDTO
	err := json.Unmarshal([]byte(`{
	  "fromToken": {"address":"0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE","symbol":"ETH","name":"Ether","decimals":18,"logoURI":""},
	  "toToken": {"address":"0xA0b86991c6218b36c1d19d4a2e9eb0ce3606eb48","symbol":"USDC","name":"USD Coin","decimals":6,"logoURI":""},
	  "fromAmount":"100",
	  "toAmount":"200",
	  "estimatedGas":12345,
	  "tx": {
	    "from":"0x1111111111111111111111111111111111111111",
	    "to":"0x2222222222222222222222222222222222222222",
	    "data":"0xabcdef",
	    "value":"0",
	    "gas": 200000,
	    "gasPrice": "1234567890"
	  }
	}`), &dto)
	require.NoError(t, err)

	out, err := AdaptSwap(1, &dto)
	require.NoError(t, err)
	require.Equal(t, "200000", out.SwapTx.Gas)
	require.Equal(t, "1234567890", out.SwapTx.GasPrice)
	require.Equal(t, "0xabcdef", out.SwapTx.Data)
}

func TestAdaptTokens_SortsOutput(t *testing.T) {
	t.Parallel()

	var dto tokensResponseDTO
	err := json.Unmarshal([]byte(`{
	  "tokens": {
	    "0x2": {"address":"0x2","symbol":"B","name":"TokenB","decimals":18,"logoURI":""},
	    "0x1": {"address":"0x1","symbol":"A","name":"TokenA","decimals":18,"logoURI":""}
	  }
	}`), &dto)
	require.NoError(t, err)

	out, err := AdaptTokens(&dto)
	require.NoError(t, err)
	require.Len(t, out, 2)
	require.Equal(t, "A", out[0].Symbol)
	require.Equal(t, "B", out[1].Symbol)
}

func TestAdaptApproveTx(t *testing.T) {
	t.Parallel()

	var dto approveTxResponseDTO
	err := json.Unmarshal([]byte(`{
	  "to":"0xA0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
	  "data":"0x095ea7b3",
	  "value":"0",
	  "gas":"50000",
	  "gasPrice": 1000000000
	}`), &dto)
	require.NoError(t, err)

	out, err := AdaptApproveTx(1, "0x1111111111111111111111111111111111111111", &dto)
	require.NoError(t, err)
	require.Equal(t, int64(1), out.ChainID)
	require.Equal(t, "0x095ea7b3", out.Data)
	require.Equal(t, "50000", out.Gas)
	require.Equal(t, "1000000000", out.GasPrice)
}
