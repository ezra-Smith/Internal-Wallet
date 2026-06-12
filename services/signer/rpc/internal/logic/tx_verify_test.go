package logic

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"

	"internalwallet/proto/pb"
)

func TestValidateConsolidationRawTx_EvmNative_OK(t *testing.T) {
	to := common.HexToAddress("0x000000000000000000000000000000000000dEaD")
	amount := big.NewInt(12345)
	unsigned := types.NewTransaction(1, to, amount, 21_000, big.NewInt(1), nil)
	raw, err := unsigned.MarshalBinary()
	require.NoError(t, err)

	req := &pb.SignTransactionRequest{
		Chain:         "ETH",
		OperationType: 3,
		ToAddress:     to.Hex(),
		Amount:        amount.String(),
	}
	require.NoError(t, validateConsolidationRawTx(req, raw))
}

func TestValidateConsolidationRawTx_EvmNative_ToMismatch_Rejected(t *testing.T) {
	to := common.HexToAddress("0x000000000000000000000000000000000000dEaD")
	amount := big.NewInt(12345)
	unsigned := types.NewTransaction(1, to, amount, 21_000, big.NewInt(1), nil)
	raw, err := unsigned.MarshalBinary()
	require.NoError(t, err)

	req := &pb.SignTransactionRequest{
		Chain:         "ETH",
		OperationType: 3,
		ToAddress:     "0x000000000000000000000000000000000000bEEF",
		Amount:        amount.String(),
	}
	require.Error(t, validateConsolidationRawTx(req, raw))
}

func TestValidateConsolidationRawTx_EvmErc20_OK(t *testing.T) {
	recipient := common.HexToAddress("0x000000000000000000000000000000000000dEaD")
	token := common.HexToAddress("0x000000000000000000000000000000000000bEEF")
	amount := big.NewInt(888)

	data := make([]byte, 4+32+32)
	copy(data[:4], []byte{0xa9, 0x05, 0x9c, 0xbb})
	copy(data[4+12:4+32], recipient.Bytes())
	amount.FillBytes(data[4+32 : 4+32+32])

	unsigned := types.NewTransaction(7, token, big.NewInt(0), 60_000, big.NewInt(1), data)
	raw, err := unsigned.MarshalBinary()
	require.NoError(t, err)

	req := &pb.SignTransactionRequest{
		Chain:         "ETH",
		OperationType: 3,
		ToAddress:     recipient.Hex(),
		Amount:        amount.String(),
		TokenContract: token.Hex(),
	}
	require.NoError(t, validateConsolidationRawTx(req, raw))
}

func TestValidateConsolidationRawTx_EvmErc20_RecipientMismatch_Rejected(t *testing.T) {
	recipient := common.HexToAddress("0x000000000000000000000000000000000000dEaD")
	token := common.HexToAddress("0x000000000000000000000000000000000000bEEF")
	amount := big.NewInt(888)

	data := make([]byte, 4+32+32)
	copy(data[:4], []byte{0xa9, 0x05, 0x9c, 0xbb})
	copy(data[4+12:4+32], recipient.Bytes())
	amount.FillBytes(data[4+32 : 4+32+32])

	unsigned := types.NewTransaction(7, token, big.NewInt(0), 60_000, big.NewInt(1), data)
	raw, err := unsigned.MarshalBinary()
	require.NoError(t, err)

	req := &pb.SignTransactionRequest{
		Chain:         "ETH",
		OperationType: 3,
		ToAddress:     "0x0000000000000000000000000000000000000001",
		Amount:        amount.String(),
		TokenContract: token.Hex(),
	}
	require.Error(t, validateConsolidationRawTx(req, raw))
}
