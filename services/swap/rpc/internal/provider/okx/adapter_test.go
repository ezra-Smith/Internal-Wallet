package okx

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdaptQuote(t *testing.T) {
	t.Parallel()

	var dto quoteDataDTO
	err := json.Unmarshal([]byte(`{
	  "chainIndex": "1",
	  "fromTokenAmount": "1000000000000000000",
	  "toTokenAmount": "2500000000",
	  "estimateGas": "21000",
	  "dexRouterList": [
	    {
	      "router": "0xrouter",
	      "percent": "1",
	      "fromToken": {"decimal":"18","tokenContractAddress":"0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE","tokenSymbol":"ETH"},
	      "toToken": {"decimal":"6","tokenContractAddress":"0xA0b86991c6218b36c1d19d4a2e9eb0ce3606eb48","tokenSymbol":"USDC"}
	    }
	  ]
	}`), &dto)
	require.NoError(t, err)

	out, err := AdaptQuote(1, "0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE", "0xA0b86991c6218b36c1d19d4a2e9eb0ce3606eb48", &dto)
	require.NoError(t, err)
	require.Equal(t, int64(1), out.ChainID)
	require.Equal(t, "okx", out.Provider)
	require.Equal(t, "ETH", out.FromToken.Symbol)
	require.Equal(t, uint32(18), out.FromToken.Decimals)
	require.Equal(t, "USDC", out.ToToken.Symbol)
	require.Equal(t, uint32(6), out.ToToken.Decimals)
	require.Equal(t, "1000000000000000000", out.FromAmount)
	require.Equal(t, "2500000000", out.ToAmount)
	require.Equal(t, uint64(21000), out.EstimatedGas)
}

func TestAdaptSwap_GasFields(t *testing.T) {
	t.Parallel()

	var dto swapDataDTO
	err := json.Unmarshal([]byte(`{
	  "chainIndex": "1",
	  "dexContractAddress": "0xdex",
	  "routerResult": {
	    "fromToken": {"decimal":"18","tokenContractAddress":"0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE","tokenSymbol":"ETH"},
	    "toToken": {"decimal":"6","tokenContractAddress":"0xA0b86991c6218b36c1d19d4a2e9eb0ce3606eb48","tokenSymbol":"USDC"},
	    "fromTokenAmount":"100",
	    "toTokenAmount":"200",
	    "dexRouterList":[]
	  },
	  "tx": {
	    "from":"0x1111111111111111111111111111111111111111",
	    "to":"0x2222222222222222222222222222222222222222",
	    "data":"0xabcdef",
	    "value":"0",
	    "gas":"690534",
	    "gasPrice":"1234567890"
	  }
	}`), &dto)
	require.NoError(t, err)

	out, err := AdaptSwap(1, &dto)
	require.NoError(t, err)
	require.Equal(t, "okx", out.Provider)
	require.Equal(t, "690534", out.SwapTx.Gas)
	require.Equal(t, "1234567890", out.SwapTx.GasPrice)
	require.Equal(t, "0xabcdef", out.SwapTx.Data)
	require.Equal(t, uint64(690534), out.Quote.EstimatedGas)
}

func TestAdaptApproveTx(t *testing.T) {
	t.Parallel()

	var dto approveTxDataDTO
	err := json.Unmarshal([]byte(`{
	  "chainIndex":"1",
	  "data":"0x095ea7b3",
	  "dexContractAddress":"0xrouter",
	  "gasLimit":"50000",
	  "gasPrice": 1000000000
	}`), &dto)
	require.NoError(t, err)

	out, err := AdaptApproveTx(1, "0x1111111111111111111111111111111111111111", "0xA0b86991c6218b36c1d19d4a2e9eb0ce3606eb48", &dto)
	require.NoError(t, err)
	require.Equal(t, int64(1), out.ChainID)
	require.Equal(t, "0x095ea7b3", out.Data)
	require.Equal(t, "50000", out.Gas)
	require.Equal(t, "1000000000", out.GasPrice)
	// To 字段应该是代币合约地址（ERC20 标准）
	require.Equal(t, "0xA0b86991c6218b36c1d19d4a2e9eb0ce3606eb48", out.To)
}

func TestAdaptTokens_SortsOutput(t *testing.T) {
	t.Parallel()

	var dto []tokenDTO
	err := json.Unmarshal([]byte(`[
	  {"decimals":"18","tokenContractAddress":"0x2","tokenSymbol":"B","tokenName":"TokenB","tokenLogoUrl":""},
	  {"decimals":"18","tokenContractAddress":"0x1","tokenSymbol":"A","tokenName":"TokenA","tokenLogoUrl":""}
	]`), &dto)
	require.NoError(t, err)

	out, err := AdaptTokens(dto)
	require.NoError(t, err)
	require.Len(t, out, 2)
	require.Equal(t, "A", out[0].Symbol)
	require.Equal(t, "B", out[1].Symbol)
}
