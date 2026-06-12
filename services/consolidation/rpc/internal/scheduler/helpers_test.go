package scheduler

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
	"internalwallet/proto/pb"
	"internalwallet/services/consolidation/rpc/internal/chainutil"
)

func TestChainStringToEnum(t *testing.T) {
	tp, err := chainutil.ChainStringToEnum("TRON")
	require.NoError(t, err)
	require.Equal(t, pb.ChainRpcType_CHAIN_TYPE_TRON, tp)

	tp, err = chainutil.ChainStringToEnum("eth")
	require.NoError(t, err)
	require.Equal(t, pb.ChainRpcType_CHAIN_TYPE_ETHEREUM, tp)

	tp, err = chainutil.ChainStringToEnum("BSC")
	require.NoError(t, err)
	require.Equal(t, pb.ChainRpcType_CHAIN_TYPE_BSC, tp)

	_, err = chainutil.ChainStringToEnum("SOL")
	require.Error(t, err)
}

func TestMulCeilBigInt(t *testing.T) {
	v, err := mulCeilBigInt(big.NewInt(10), 1.2)
	require.NoError(t, err)
	require.Equal(t, "12", v.String())

	v, err = mulCeilBigInt(big.NewInt(5), 1.1)
	require.NoError(t, err)
	require.Equal(t, "6", v.String())
}

func TestRetryDelaySeconds(t *testing.T) {
	require.Equal(t, int64(60), retryDelaySeconds([]int64{60, 300, 900}, 1))
	require.Equal(t, int64(300), retryDelaySeconds([]int64{60, 300, 900}, 2))
	require.Equal(t, int64(900), retryDelaySeconds([]int64{60, 300, 900}, 3))
	require.Equal(t, int64(900), retryDelaySeconds([]int64{60, 300, 900}, 4))

	require.Equal(t, int64(60), retryDelaySeconds(nil, 1))
	require.Equal(t, int64(300), retryDelaySeconds(nil, 2))
	require.Equal(t, int64(900), retryDelaySeconds(nil, 3))
}
