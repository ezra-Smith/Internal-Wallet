package security

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

const (
	// DefaultBcryptCost bcrypt 默认成本（推荐值：10-12）
	DefaultBcryptCost = 12
	// MinPasswordLength 最小密码长度
	MinPasswordLength = 8
)

// HashPassword 使用 bcrypt 生成密码哈希
// 这是推荐的密码哈希方法，适用于所有用户类型（普通用户、管理员）
func HashPassword(password string) (string, error) {
	if password == "" {
		return "", fmt.Errorf("password cannot be empty")
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), DefaultBcryptCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(hashed), nil
}

// VerifyPassword 验证明文密码是否匹配 bcrypt 哈希
func VerifyPassword(hashedPassword, password string) bool {
	if hashedPassword == "" || password == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password)) == nil
}

// ValidatePasswordStrength 校验密码强度
// minLen: 最小长度（建议 8-16）
// requireComplex: 是否要求复杂度（大小写+数字+特殊字符）
func ValidatePasswordStrength(password string, minLen int, requireComplex bool) error {
	if minLen <= 0 {
		minLen = MinPasswordLength
	}

	if len(password) < minLen {
		return fmt.Errorf("password too short, minimum length is %d", minLen)
	}

	if !requireComplex {
		return nil
	}

	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSpecial = true
		}
	}

	if !hasUpper || !hasLower || !hasDigit || !hasSpecial {
		return fmt.Errorf("password must contain uppercase, lowercase, digit, and special character")
	}

	return nil
}

// GenerateSecurePassword 生成满足强度要求的随机密码
func GenerateSecurePassword(length int) (string, error) {
	if length < 12 {
		length = 12
	}

	upper := "ABCDEFGHJKLMNPQRSTUVWXYZ"
	lower := "abcdefghijkmnopqrstuvwxyz"
	digits := "23456789"
	special := "!@#$%^&*"
	all := upper + lower + digits + special

	// 确保每类至少 1 个字符
	buf := make([]byte, 0, length)

	pick := func(chars string) (byte, error) {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			return 0, err
		}
		return chars[n.Int64()], nil
	}

	// 至少一个大写
	u, err := pick(upper)
	if err != nil {
		return "", err
	}
	// 至少一个小写
	l, err := pick(lower)
	if err != nil {
		return "", err
	}
	// 至少一个数字
	d, err := pick(digits)
	if err != nil {
		return "", err
	}
	// 至少一个特殊字符
	s, err := pick(special)
	if err != nil {
		return "", err
	}

	buf = append(buf, u, l, d, s)

	// 填充剩余字符
	for len(buf) < length {
		ch, err := pick(all)
		if err != nil {
			return "", err
		}
		buf = append(buf, ch)
	}

	// Fisher-Yates 随机洗牌
	for i := len(buf) - 1; i > 0; i-- {
		jBig, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return "", err
		}
		j := int(jBig.Int64())
		buf[i], buf[j] = buf[j], buf[i]
	}

	return string(buf), nil
}

// IsLegacyHash 检查是否为旧的 SHA-256 哈希（64字符十六进制）
// 用于迁移逻辑，识别需要升级的旧密码
func IsLegacyHash(hash string) bool {
	// bcrypt 哈希以 "$2a$", "$2b$", "$2y$" 开头
	if len(hash) > 4 && hash[0] == '$' && hash[1] == '2' {
		return false
	}
	// SHA-256 哈希是 64 字符的十六进制字符串
	if len(hash) == 64 {
		for _, c := range hash {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return false
			}
		}
		return true
	}
	return false
}
