package logic

import "strings"

// normalizeWeb3Address normalizes addresses for consistent storage and lookups.
//
// Rules:
// - EVM (0x...) addresses are case-insensitive; store/use lower-case to match chainsync normalization.
// - Non-0x addresses (e.g. TRON base58) are treated as case-sensitive and kept as-is.
func normalizeWeb3Address(_ int64, address string) string {
	address = strings.TrimSpace(address)
	if address == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(address), "0x") {
		return strings.ToLower(address)
	}
	return address
}

