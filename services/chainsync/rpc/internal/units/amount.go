package units

import (
	"math/big"
	"strings"
)

// FormatTokenAmount formats a base-unit integer string into a decimal string with `decimals`.
// Example: amount="123450000", decimals=6 => "123.45".
func FormatTokenAmount(amount string, decimals int) string {
	if strings.TrimSpace(amount) == "" {
		return "0"
	}

	amt := new(big.Int)
	if _, ok := amt.SetString(amount, 10); !ok {
		return amount
	}

	if amt.Sign() == 0 {
		return "0"
	}

	divisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	integer := new(big.Int).Div(amt, divisor)
	remainder := new(big.Int).Mod(amt, divisor)

	result := integer.String()
	if remainder.Sign() == 0 {
		return result
	}

	decimalStr := remainder.String()
	for len(decimalStr) < decimals {
		decimalStr = "0" + decimalStr
	}
	decimalStr = strings.TrimRight(decimalStr, "0")
	if decimalStr == "" {
		return result
	}
	return result + "." + decimalStr
}

