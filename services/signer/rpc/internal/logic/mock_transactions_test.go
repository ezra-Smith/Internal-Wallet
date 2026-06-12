package logic

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func CreateMockETHTransaction() ([]byte, error) {
	return createMockEvmLegacyTransaction()
}

func CreateMockBSCTransaction() ([]byte, error) {
	return createMockEvmLegacyTransaction()
}

func createMockEvmLegacyTransaction() ([]byte, error) {
	to := common.HexToAddress("0x000000000000000000000000000000000000dEaD")
	tx := types.NewTransaction(
		0,  // nonce
		to, // to
		big.NewInt(0),
		21000,
		big.NewInt(1_000_000_000), // 1 gwei
		nil,
	)
	return tx.MarshalBinary()
}
