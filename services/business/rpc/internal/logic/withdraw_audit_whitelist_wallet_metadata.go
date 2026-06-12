package logic

import (
	"encoding/base64"
	"errors"
	"strings"
	"unicode/utf8"
)

const (
	withdrawAuditWhitelistWalletNameMaxRunes = 50
	// 兼容客户端头像 data-uri：放宽上限；同时“超限/非法”不再导致整单失败（见 normalize*）
	withdrawAuditWhitelistWalletIconMaxBytes = 1024 * 1024  // 1MB
	withdrawAuditWhitelistWalletIconMaxChars = 1600 * 1024  // 1.6MB chars
)

var withdrawAuditWhitelistWalletIconAllowedMimes = map[string]struct{}{
	"image/png":  {},
	"image/jpeg": {},
	"image/webp": {},
}

func normalizeWithdrawAuditWhitelistWalletNameRequired(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("wallet_name is required")
	}
	if utf8.RuneCountInString(name) > withdrawAuditWhitelistWalletNameMaxRunes {
		return "", errors.New("wallet_name too long")
	}
	return name, nil
}

func normalizeWithdrawAuditWhitelistWalletIcon(icon string) (string, error) {
	icon = strings.TrimSpace(icon)
	if icon == "" {
		return "", nil
	}
	if len(icon) > withdrawAuditWhitelistWalletIconMaxChars {
		// 宽容：超大头像直接忽略，避免触发 INVALID_PARAM
		return "", nil
	}
	if strings.HasPrefix(icon, "data:") {
		return normalizeWithdrawAuditWhitelistWalletIconDataURI(icon)
	}

	// raw base64
	payload := stripBase64Whitespace(icon)
	data, err := decodeBase64(payload)
	if err != nil {
		// 宽容：非法 base64 直接忽略
		return "", nil
	}
	if len(data) > withdrawAuditWhitelistWalletIconMaxBytes {
		// 宽容：超大头像直接忽略
		return "", nil
	}
	mime, ok := detectImageMime(data)
	if !ok {
		// 宽容：无法识别的内容直接忽略
		return "", nil
	}
	if _, ok := withdrawAuditWhitelistWalletIconAllowedMimes[mime]; !ok {
		// 宽容：不支持的 mime 直接忽略
		return "", nil
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

func normalizeWithdrawAuditWhitelistWalletIconDataURI(icon string) (string, error) {
	parts := strings.SplitN(icon, ",", 2)
	if len(parts) != 2 {
		return "", nil
	}
	header := strings.TrimSpace(parts[0])
	payload := stripBase64Whitespace(parts[1])

	if !strings.HasPrefix(header, "data:") || !strings.Contains(header, ";base64") {
		return "", nil
	}

	header = strings.TrimPrefix(header, "data:")
	semiIdx := strings.Index(header, ";")
	if semiIdx <= 0 {
		return "", nil
	}
	mime := strings.ToLower(strings.TrimSpace(header[:semiIdx]))
	if mime == "image/jpg" {
		mime = "image/jpeg"
	}
	if _, ok := withdrawAuditWhitelistWalletIconAllowedMimes[mime]; !ok {
		return "", nil
	}

	data, err := decodeBase64(payload)
	if err != nil {
		return "", nil
	}
	if len(data) > withdrawAuditWhitelistWalletIconMaxBytes {
		return "", nil
	}
	actualMime, ok := detectImageMime(data)
	if !ok || actualMime != mime {
		return "", nil
	}

	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

func stripBase64Whitespace(s string) string {
	// base64 decoder does not ignore whitespace, so strip common whitespace chars.
	return strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\n', '\r', '\t':
			return -1
		default:
			return r
		}
	}, s)
}

func decodeBase64(s string) ([]byte, error) {
	if s == "" {
		return nil, errors.New("empty base64")
	}
	if b, err := base64.StdEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	if b, err := base64.RawStdEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	return nil, errors.New("invalid base64")
}

func detectImageMime(data []byte) (string, bool) {
	if len(data) >= 8 &&
		data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 &&
		data[4] == 0x0D && data[5] == 0x0A && data[6] == 0x1A && data[7] == 0x0A {
		return "image/png", true
	}
	if len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "image/jpeg", true
	}
	if len(data) >= 12 &&
		data[0] == 'R' && data[1] == 'I' && data[2] == 'F' && data[3] == 'F' &&
		data[8] == 'W' && data[9] == 'E' && data[10] == 'B' && data[11] == 'P' {
		return "image/webp", true
	}
	return "", false
}
