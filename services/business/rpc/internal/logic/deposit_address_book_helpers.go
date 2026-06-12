package logic

import (
	"strings"
	"time"
)

func trimToNil(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

func formatTimePtr(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.Format("2006/01/02 15:04:05")
}

func depositAddressStatKey(chainCode string, address string) string {
	chainCode = strings.ToUpper(strings.TrimSpace(chainCode))
	address = strings.TrimSpace(address)
	if strings.HasPrefix(strings.ToLower(address), "0x") {
		return chainCode + "|" + strings.ToLower(address)
	}
	return chainCode + "|" + address
}
