package utils

import (
	"math/big"
	"strings"
)

// FormatWeb3Fee returns a human-readable native fee string and its asset symbol.
//
// Inputs:
// - chainCode: "ETH" / "BSC" / "TRON" (case-insensitive)
// - gasFee: may be
//   - already formatted like "0.00001 ETH"
//   - decimal like "0.00001" (asset unit)
//   - raw integer like "821036580000" (wei/sun)
// - gasUsed + gasPrice: fallback inputs to compute fee when gasFee is empty (wei/sun)
//
// Output:
// - fee: formatted string like "0.00001 ETH" (nil if unavailable)
// - feeAsset: native symbol ("ETH"/"BNB"/"TRX") as pointer (nil if chainCode unsupported)
func FormatWeb3Fee(chainCode string, gasFee string, gasUsed uint64, gasPrice string) (fee *string, feeAsset *string) {
	asset := nativeFeeAsset(chainCode)
	if asset != "" {
		feeAsset = StringPtr(asset)
	}

	gasFee = strings.TrimSpace(gasFee)
	if gasFee != "" {
		// Already formatted with unit (e.g. "0.00001 ETH").
		if len(strings.Fields(gasFee)) >= 2 {
			return StringPtr(gasFee), feeAsset
		}

		// Decimal in asset units (no unit attached yet).
		if strings.Contains(gasFee, ".") {
			return StringPtr(gasFee + " " + asset), feeAsset
		}

		// Raw integer (wei/sun) -> convert to decimal.
		if raw := parseBigIntLoose(gasFee); raw != nil && raw.Sign() >= 0 {
			decimals := nativeFeeDecimals(chainCode)
			return StringPtr(formatAmountFromRawInt(raw, decimals) + " " + asset), feeAsset
		}

		// Best-effort fallback: return as-is (append unit to keep UI readable).
		return StringPtr(gasFee + " " + asset), feeAsset
	}

	// Fallback: compute fee from gas_used * gas_price (wei/sun).
	if gasUsed > 0 {
		gp := parseBigIntLoose(strings.TrimSpace(gasPrice))
		if gp != nil && gp.Sign() > 0 {
			rawFee := new(big.Int).Mul(gp, new(big.Int).SetUint64(gasUsed))
			if rawFee.Sign() > 0 {
				decimals := nativeFeeDecimals(chainCode)
				return StringPtr(formatAmountFromRawInt(rawFee, decimals) + " " + asset), feeAsset
			}
		}
	}

	return nil, feeAsset
}

func nativeFeeAsset(chainCode string) string {
	switch strings.ToUpper(strings.TrimSpace(chainCode)) {
	case "ETH", "ETHEREUM":
		return "ETH"
	case "BSC", "BNB":
		return "BNB"
	case "TRON", "TRX":
		return "TRX"
	default:
		return ""
	}
}

func nativeFeeDecimals(chainCode string) int {
	switch strings.ToUpper(strings.TrimSpace(chainCode)) {
	case "TRON", "TRX":
		return 6
	default:
		return 18
	}
}

func parseBigIntLoose(s string) *big.Int {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	bi := new(big.Int)
	if strings.HasPrefix(strings.ToLower(s), "0x") {
		if _, ok := bi.SetString(strings.TrimPrefix(strings.ToLower(s), "0x"), 16); ok {
			return bi
		}
		return nil
	}
	if _, ok := bi.SetString(s, 10); ok {
		return bi
	}
	return nil
}

func formatAmountFromRawInt(raw *big.Int, decimals int) string {
	if raw == nil || raw.Sign() == 0 {
		return "0"
	}
	if decimals <= 0 {
		return raw.String()
	}

	divisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	intPart := new(big.Int).Div(raw, divisor)
	rem := new(big.Int).Mod(raw, divisor)
	if rem.Sign() == 0 {
		return intPart.String()
	}

	remStr := rem.String()
	if len(remStr) < decimals {
		remStr = strings.Repeat("0", decimals-len(remStr)) + remStr
	}
	remStr = strings.TrimRight(remStr, "0")
	if remStr == "" {
		return intPart.String()
	}
	return intPart.String() + "." + remStr
}

