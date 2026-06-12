package accounting

import (
	"fmt"
	"strings"

	"github.com/shopspring/decimal"
)

func NormalizeCode(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}

func NormalizeChainCode(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}

func NormalizeAssetCode(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}

func ParseDecimalToRawExact(amountDecimal string, scale int32, allowZero bool) (raw decimal.Decimal, err error) {
	amountDecimal = strings.TrimSpace(amountDecimal)
	if amountDecimal == "" {
		return decimal.Zero, fmt.Errorf("amount required")
	}
	if strings.ContainsAny(amountDecimal, "eE") {
		return decimal.Zero, fmt.Errorf("scientific notation not allowed")
	}
	if scale < 0 || scale > 30 {
		return decimal.Zero, fmt.Errorf("invalid scale")
	}

	amt, err := decimal.NewFromString(amountDecimal)
	if err != nil {
		return decimal.Zero, fmt.Errorf("invalid amount format")
	}
	if allowZero {
		if amt.LessThan(decimal.Zero) {
			return decimal.Zero, fmt.Errorf("amount must be >= 0")
		}
	} else {
		if amt.LessThanOrEqual(decimal.Zero) {
			return decimal.Zero, fmt.Errorf("amount must be > 0")
		}
	}

	// Precision check: reject implicit rounding.
	if amt.Exponent() < -scale {
		return decimal.Zero, fmt.Errorf("amount exceeds maximum precision of %d decimals", scale)
	}

	raw = amt.Shift(scale)
	if !raw.Equal(raw.Truncate(0)) {
		return decimal.Zero, fmt.Errorf("amount cannot be represented in smallest unit")
	}
	raw = raw.Truncate(0)

	// Final exactness & bounds checks.
	if raw.LessThan(decimal.Zero) {
		return decimal.Zero, fmt.Errorf("raw must be >= 0")
	}
	if !allowZero && raw.Equal(decimal.Zero) {
		return decimal.Zero, fmt.Errorf("amount must be > 0")
	}
	if err := ValidateDecimal65Int(raw); err != nil {
		return decimal.Zero, err
	}
	return raw, nil
}

func ValidateDecimal65Int(v decimal.Decimal) error {
	if v.Exponent() != 0 {
		// Note: decimal may carry exp!=0 for integers (e.g. 1e6); compare via String.
		// We validate by string form to avoid false negatives.
	}
	if v.LessThan(decimal.Zero) {
		return fmt.Errorf("must be >= 0")
	}
	s := v.String()
	if strings.HasPrefix(s, "-") {
		s = strings.TrimPrefix(s, "-")
	}
	s = strings.TrimLeft(s, "0")
	if s == "" {
		s = "0"
	}
	if strings.Contains(s, ".") {
		return fmt.Errorf("must be integer")
	}
	if len(s) > 65 {
		return fmt.Errorf("amount too large")
	}
	return nil
}

func RawToDecimalString(raw decimal.Decimal, scale int32) (string, error) {
	if raw.LessThan(decimal.Zero) {
		return "", fmt.Errorf("raw must be >= 0")
	}
	if scale < 0 || scale > 30 {
		return "", fmt.Errorf("invalid scale")
	}
	if err := ValidateDecimal65Int(raw.Truncate(0)); err != nil {
		return "", err
	}
	// Fixed to asset precision for stable representation.
	return raw.Shift(-scale).StringFixed(scale), nil
}
