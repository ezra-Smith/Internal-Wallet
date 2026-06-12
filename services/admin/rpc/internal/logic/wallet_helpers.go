package logic

import (
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

func trimOrEmpty(s string) string { return strings.TrimSpace(s) }

func strPtrOrNilTrim(s string) *string {
	v := strings.TrimSpace(s)
	if v == "" {
		return nil
	}
	return &v
}

func parseCSV(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}

func maskAddress(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if len(s) <= 10 {
		return "****"
	}
	return s[:6] + "****" + s[len(s)-4:]
}

func chainCodeToChainType(chainCode string) string {
	switch strings.ToUpper(strings.TrimSpace(chainCode)) {
	case "ETH":
		return "ethereum"
	case "TRON":
		return "tron"
	case "BTC":
		return "bitcoin"
	case "BSC":
		return "bsc"
	case "POLYGON":
		return "polygon"
	case "ARBITRUM":
		return "arbitrum"
	case "OPTIMISM":
		return "optimism"
	default:
		return strings.ToLower(strings.TrimSpace(chainCode))
	}
}

func chainTypeToChainCode(chainType string) string {
	switch strings.ToLower(strings.TrimSpace(chainType)) {
	case "ethereum", "ETH":
		return "ETH"
	case "tron":
		return "TRON"
	case "bitcoin":
		return "BTC"
	case "bsc":
		return "BSC"
	case "polygon":
		return "POLYGON"
	case "arbitrum":
		return "ARBITRUM"
	case "optimism":
		return "OPTIMISM"
	default:
		return strings.ToUpper(strings.TrimSpace(chainType))
	}
}

func nativeAssetCodeForChainCode(chainCode string) string {
	switch strings.ToUpper(strings.TrimSpace(chainCode)) {
	case "ETH":
		return "ETH"
	case "TRON":
		return "TRX"
	case "BTC":
		return "BTC"
	case "BSC":
		return "BNB"
	case "POLYGON":
		return "MATIC"
	case "ARBITRUM", "OPTIMISM":
		return "ETH"
	default:
		return ""
	}
}

// Deposit address status constants
const (
	DepositAddressStatusActive     = "active"     // 活跃，可充值
	DepositAddressStatusInactive   = "inactive"   // 不活跃
	DepositAddressStatusDeprecated = "deprecated" // 废弃
)

func validateDepositAddressStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case DepositAddressStatusActive, DepositAddressStatusInactive, DepositAddressStatusDeprecated:
		return true
	default:
		return false
	}
}

func validateDepositStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case "pending", "confirmed", "completed", "failed":
		return true
	default:
		return false
	}
}

func validateWithdrawalStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case "pending", "processing", "completed", "failed", "cancelled", "rejected":
		return true
	default:
		return false
	}
}

func validateChainType(chainType string) bool {
	return strings.TrimSpace(chainType) != ""
}

func validateEVMAddress(addr string) bool {
	evmAddr := regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`)
	return evmAddr.MatchString(strings.TrimSpace(addr))
}

func validateTronAddress(addr string) bool {
	// Basic Base58Check-like format: Tron mainnet addresses typically start with 'T' and length 34.
	addr = strings.TrimSpace(addr)
	if len(addr) != 34 || !strings.HasPrefix(addr, "T") {
		return false
	}
	// Exclude 0/O/I/l characters as base58 does.
	tronRe := regexp.MustCompile(`^T[1-9A-HJ-NP-Za-km-z]{33}$`)
	return tronRe.MatchString(addr)
}

func validateBitcoinAddress(addr string) bool {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return false
	}
	// Accept common base58 and bech32 patterns (basic validation only).
	base58 := regexp.MustCompile(`^[13][1-9A-HJ-NP-Za-km-z]{25,34}$`)
	bech32 := regexp.MustCompile(`^(bc1|tb1)[0-9a-z]{11,71}$`)
	return base58.MatchString(addr) || bech32.MatchString(strings.ToLower(addr))
}

func validateChainAddress(chainType string, address string) error {
	chainType = strings.ToLower(strings.TrimSpace(chainType))
	address = strings.TrimSpace(address)
	if address == "" {
		return fmt.Errorf("empty address")
	}
	if len(address) > 255 {
		return fmt.Errorf("address too long")
	}
	switch chainType {
	case "ethereum", "bsc", "polygon", "arbitrum", "optimism":
		if !validateEVMAddress(address) {
			return fmt.Errorf("invalid evm address")
		}
	case "tron":
		if !validateTronAddress(address) {
			return fmt.Errorf("invalid tron address")
		}
	case "bitcoin":
		if !validateBitcoinAddress(address) {
			return fmt.Errorf("invalid bitcoin address")
		}
	default:
		// Unknown chain type: only do basic non-empty validation.
	}
	return nil
}

func pow10(n int32) *big.Int {
	if n <= 0 {
		return big.NewInt(1)
	}
	out := big.NewInt(1)
	ten := big.NewInt(10)
	for i := int32(0); i < n; i++ {
		out.Mul(out, ten)
	}
	return out
}

func decimalToInt64Exact(d decimal.Decimal) (int64, error) {
	coeff := new(big.Int).Set(d.Coefficient())
	exp := d.Exponent()
	switch {
	case exp > 0:
		coeff.Mul(coeff, pow10(exp))
	case exp < 0:
		div := pow10(-exp)
		rem := new(big.Int)
		coeff.DivMod(coeff, div, rem)
		if rem.Sign() != 0 {
			return 0, fmt.Errorf("not an integer")
		}
	}
	if !coeff.IsInt64() {
		return 0, fmt.Errorf("overflow int64")
	}
	return coeff.Int64(), nil
}

type amountParseResult struct {
	AmountStr string
	Raw       int64
}

func parseAmountToDecimalString(amountStr string, tokenDecimals int32, allowZero bool) (string, error) {
	amountStr = strings.TrimSpace(amountStr)
	if amountStr == "" {
		return "", fmt.Errorf("amount required")
	}
	if tokenDecimals < 0 || tokenDecimals > 30 {
		return "", fmt.Errorf("invalid token decimals")
	}
	amt, err := decimal.NewFromString(amountStr)
	if err != nil {
		return "", fmt.Errorf("invalid amount format")
	}
	if allowZero {
		if amt.LessThan(decimal.Zero) {
			return "", fmt.Errorf("amount must be >= 0")
		}
	} else {
		if amt.LessThanOrEqual(decimal.Zero) {
			return "", fmt.Errorf("amount must be > 0")
		}
	}
	if amt.Exponent() < -tokenDecimals {
		return "", fmt.Errorf("amount exceeds maximum precision of %d decimals", tokenDecimals)
	}
	return amt.String(), nil
}

func parseAmountToRaw(amountStr string, tokenDecimals int32, allowZero bool) (amountParseResult, error) {
	amountStr = strings.TrimSpace(amountStr)
	if amountStr == "" {
		return amountParseResult{}, fmt.Errorf("amount required")
	}
	if tokenDecimals < 0 || tokenDecimals > 30 {
		return amountParseResult{}, fmt.Errorf("invalid token decimals")
	}

	amt, err := decimal.NewFromString(amountStr)
	if err != nil {
		return amountParseResult{}, fmt.Errorf("invalid amount format")
	}

	if allowZero {
		if amt.LessThan(decimal.Zero) {
			return amountParseResult{}, fmt.Errorf("amount must be >= 0")
		}
	} else {
		if amt.LessThanOrEqual(decimal.Zero) {
			return amountParseResult{}, fmt.Errorf("amount must be > 0")
		}
	}

	// Precision check (max decimals).
	if amt.Exponent() < -tokenDecimals {
		return amountParseResult{}, fmt.Errorf("amount exceeds maximum precision of %d decimals", tokenDecimals)
	}

	rawDec := amt.Shift(tokenDecimals)
	// Ensure exact integer after scaling.
	if !rawDec.Equal(rawDec.Truncate(0)) {
		return amountParseResult{}, fmt.Errorf("amount cannot be represented in smallest unit")
	}
	raw, err := decimalToInt64Exact(rawDec)
	if err != nil {
		return amountParseResult{}, err
	}
	if !allowZero && raw <= 0 {
		return amountParseResult{}, fmt.Errorf("amount must be > 0")
	}
	if allowZero && raw < 0 {
		return amountParseResult{}, fmt.Errorf("amount must be >= 0")
	}

	normalized := amt.StringFixed(tokenDecimals)
	return amountParseResult{AmountStr: normalized, Raw: raw}, nil
}

func parseDateFromTo(dateFrom, dateTo string) (*time.Time, *time.Time, error) {
	from, err := parseDateStart(dateFrom)
	if err != nil {
		return nil, nil, err
	}
	to, err := parseDateEnd(dateTo)
	if err != nil {
		return nil, nil, err
	}
	return from, to, nil
}

// formatRawBalanceWithPrecision 将原始余额（整数字符串）转换为格式化余额（小数字符串）
// rawBalance: 原始余额字符串（如 "21070000"）
// precision: 代币精度（如 6 表示 USDT）
// 返回格式化余额（如 "21.07"）
func formatRawBalanceWithPrecision(rawBalance string, precision int32) string {
	if rawBalance == "" || rawBalance == "0" {
		return "0"
	}

	// 使用 decimal 库处理精度
	rawDec, err := decimal.NewFromString(rawBalance)
	if err != nil {
		return "0"
	}

	if precision <= 0 {
		return rawDec.String()
	}

	// 除以 10^precision
	formatted := rawDec.Shift(-precision)
	return formatted.String()
}
