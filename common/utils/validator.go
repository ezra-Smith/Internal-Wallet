package utils

import (
	"net/mail"
	"regexp"
	"strings"
)

// ValidateEmail 验证邮箱格式
func ValidateEmail(email string) bool {
	email = strings.TrimSpace(email)
	if email == "" {
		return false
	}
	// RFC 5322-ish validation via net/mail.
	// ParseAddress accepts display names (e.g. "Name <a@b.com>"), so require the raw input to be the address.
	addr, err := mail.ParseAddress(email)
	if err != nil {
		return false
	}
	return addr.Address == email
}

// ValidatePhone 验证手机号格式
func ValidatePhone(phone string) bool {
	if len(phone) < 4 || len(phone) > 15 {
		return false
	}
	pattern := `^[0-9]+$`
	matched, _ := regexp.MatchString(pattern, phone)
	return matched
}

// ValidatePassword 验证密码强度
func ValidatePassword(password string) bool {
	// 至少8位，包含字母和数字
	if len(password) < 8 {
		return false
	}
	hasLetter := regexp.MustCompile(`[a-zA-Z]`).MatchString(password)
	hasDigit := regexp.MustCompile(`\d`).MatchString(password)
	return hasLetter && hasDigit
}
