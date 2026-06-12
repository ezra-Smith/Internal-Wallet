package logic

import "strings"

func isValidWalletAddress(chainCode, address string) bool {
	chainCode = normalizeCode(chainCode)
	address = strings.TrimSpace(address)
	if address == "" {
		return false
	}

	switch chainCode {
	case "ETH", "BSC", "OP", "ARB", "BASE":
		return strings.HasPrefix(address, "0x") && len(address) == 42
	case "TRN", "TRON", "TRX":
		return strings.HasPrefix(address, "T") && len(address) >= 34 && len(address) <= 36
	default:
		return len(address) >= 20
	}
}
