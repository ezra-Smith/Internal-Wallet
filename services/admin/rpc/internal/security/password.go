package security

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

const (
	DefaultBcryptCost = 12
)

// HashPassword 使用 bcrypt 生成密码哈希
func HashPassword(password string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), DefaultBcryptCost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

// VerifyPassword 验证明文密码是否匹配 bcrypt 哈希
func VerifyPassword(passwordHash string, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)) == nil
}

// ValidatePasswordPolicy 校验管理员密码强度（对齐 SRD）
func ValidatePasswordPolicy(password string, minLen int) error {
	if minLen <= 0 {
		minLen = 10
	}
	if len(password) < minLen {
		return fmt.Errorf("password too short, min length %d", minLen)
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
		default:
			// special：包含常见符号即可
			hasSpecial = true
		}
	}

	if !hasUpper || !hasLower || !hasDigit || !hasSpecial {
		return fmt.Errorf("password too weak: require upper/lower/digit/special")
	}
	return nil
}

// GenerateTempPassword 生成满足策略的临时密码
func GenerateTempPassword(length int) (string, error) {
	if length < 12 {
		length = 12
	}

	upper := "ABCDEFGHJKLMNPQRSTUVWXYZ"
	lower := "abcdefghijkmnopqrstuvwxyz"
	digits := "23456789"
	special := "!@#$%^&*"
	all := upper + lower + digits + special

	// 确保每类至少1个
	buf := make([]byte, 0, length)
	pick := func(chars string) (byte, error) {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			return 0, err
		}
		return chars[n.Int64()], nil
	}

	u, err := pick(upper)
	if err != nil {
		return "", err
	}
	l, err := pick(lower)
	if err != nil {
		return "", err
	}
	d, err := pick(digits)
	if err != nil {
		return "", err
	}
	s, err := pick(special)
	if err != nil {
		return "", err
	}

	buf = append(buf, u, l, d, s)
	for len(buf) < length {
		ch, pickErr := pick(all)
		if pickErr != nil {
			return "", pickErr
		}
		buf = append(buf, ch)
	}

	// Fisher–Yates shuffle
	for i := len(buf) - 1; i > 0; i-- {
		jBig, pickErr := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if pickErr != nil {
			return "", pickErr
		}
		j := int(jBig.Int64())
		buf[i], buf[j] = buf[j], buf[i]
	}

	return string(buf), nil
}
