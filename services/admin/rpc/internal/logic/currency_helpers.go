package logic

import (
	"strings"

	"github.com/shopspring/decimal"
)

const baseCurrencyCode = "USDT"

func normalizeCode(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}

func enforceBaseCurrencySwapEnabled(assetCode string, web3SwapEnabled bool) bool {
	if normalizeCode(assetCode) == baseCurrencyCode {
		return true
	}
	return web3SwapEnabled
}

func parseNonNegativeDecimalOrEmpty(s string) (decimal.Decimal, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return decimal.Zero, false
	}
	d, err := decimal.NewFromString(s)
	if err != nil || d.IsNegative() {
		return decimal.Zero, false
	}
	return d, true
}

func parseNonNegativeDecimal(s string) (decimal.Decimal, bool) {
	d, ok := parseNonNegativeDecimalOrEmpty(s)
	if !ok {
		return decimal.Zero, false
	}
	return d, true
}

func isValidFeeRuleType(t string) bool {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "fixed", "percent":
		return true
	default:
		return false
	}
}

// validateDecimalRange validates that a decimal fits within decimal(40,6) limits
// decimal(40,6) allows: 40 total digits, 6 decimal places, so max 34 integer digits
// Product standard: unified 6 decimal places for all amounts
func validateDecimalRange(d decimal.Decimal) bool {
	if d.IsNegative() {
		return false
	}

	// Get string representation and check integer part length
	str := d.String()

	// Split by decimal point
	parts := strings.Split(str, ".")
	integerPart := parts[0]

	// Remove leading zeros for accurate digit count
	integerPart = strings.TrimLeft(integerPart, "0")
	if integerPart == "" {
		integerPart = "0"
	}

	// Check if integer part exceeds 34 digits
	if len(integerPart) > 34 {
		return false
	}

	// Check decimal part if exists (max 6 digits - product standard)
	if len(parts) > 1 {
		decimalPart := parts[1]
		if len(decimalPart) > 6 {
			return false
		}
	}

	return true
}

// roundToSixDecimals rounds a decimal to 6 decimal places (product standard)
func roundToSixDecimals(d decimal.Decimal) decimal.Decimal {
	return d.Round(6)
}
