package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// VerifyTOTP 校验 TOTP（RFC 6238, HMAC-SHA1, 6位, 30秒步长）
// skew: 允许的时间窗口偏移（例如 1 表示允许前后各 1 个窗口）
func VerifyTOTP(secret string, code string, now time.Time, skew int) (bool, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return false, fmt.Errorf("empty totp secret")
	}

	normalized := strings.ToUpper(strings.ReplaceAll(secret, " ", ""))
	enc := base32.StdEncoding.WithPadding(base32.NoPadding)
	key, err := enc.DecodeString(normalized)
	if err != nil {
		// 兼容带 padding 的密钥
		key, err = base32.StdEncoding.DecodeString(normalized)
		if err != nil {
			return false, fmt.Errorf("invalid totp secret")
		}
	}

	if skew < 0 {
		skew = 0
	}
	counter := now.Unix() / 30
	for i := -skew; i <= skew; i++ {
		otp := hotp(key, uint64(counter+int64(i)), 6)
		if subtle.ConstantTimeCompare([]byte(otp), []byte(code)) == 1 {
			return true, nil
		}
	}
	return false, nil
}

func hotp(key []byte, counter uint64, digits int) string {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], counter)

	mac := hmac.New(sha1.New, key)
	_, _ = mac.Write(buf[:])
	sum := mac.Sum(nil)

	offset := sum[len(sum)-1] & 0x0f
	binCode := (int(sum[offset])&0x7f)<<24 |
		(int(sum[offset+1])&0xff)<<16 |
		(int(sum[offset+2])&0xff)<<8 |
		(int(sum[offset+3]) & 0xff)

	mod := 1
	for i := 0; i < digits; i++ {
		mod *= 10
	}
	otp := binCode % mod

	return fmt.Sprintf("%0*d", digits, otp)
}

// GenerateTOTPSecret 生成一个 Base32 编码的 TOTP secret（无 padding，默认 20 bytes）
func GenerateTOTPSecret(numBytes int) (string, error) {
	if numBytes <= 0 {
		numBytes = 20
	}
	b := make([]byte, numBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	enc := base32.StdEncoding.WithPadding(base32.NoPadding)
	return enc.EncodeToString(b), nil
}

// BuildTOTPAuthURL 构建 otpauth URL（可用于 Google Authenticator/1Password 等扫描/手动添加）
func BuildTOTPAuthURL(issuer, account, secret string) string {
	issuer = strings.TrimSpace(issuer)
	account = strings.TrimSpace(account)
	secret = strings.TrimSpace(secret)
	if issuer == "" || account == "" || secret == "" {
		return ""
	}

	label := url.PathEscape(fmt.Sprintf("%s:%s", issuer, account))
	q := url.Values{}
	q.Set("secret", secret)
	q.Set("issuer", issuer)
	q.Set("algorithm", "SHA1")
	q.Set("digits", "6")
	q.Set("period", "30")
	return fmt.Sprintf("otpauth://totp/%s?%s", label, q.Encode())
}
