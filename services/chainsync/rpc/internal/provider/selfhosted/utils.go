package selfhosted

import (
	"math/big"
	"strings"
)

func parseHexToBigInt(hexStr string) *big.Int {
	if hexStr == "" {
		return nil
	}

	result := new(big.Int)

	if strings.HasPrefix(hexStr, "0x") || strings.HasPrefix(hexStr, "0X") {
		hexStr = hexStr[2:]
	}

	_, ok := result.SetString(hexStr, 16)
	if !ok {
		return nil
	}

	return result
}

