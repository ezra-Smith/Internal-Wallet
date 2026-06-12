package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

func GenerateAPIKey() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate api key: %w", err)
	}
	// URL-safe, no padding
	return "swp_" + base64.RawURLEncoding.EncodeToString(b[:]), nil
}

// HashAPIKey returns the deterministic hash for a plaintext API key.
// We use HMAC-SHA256(pepper, apiKey) and encode as lowercase hex.
func HashAPIKey(pepper string, apiKey string) (string, error) {
	pepper = strings.TrimSpace(pepper)
	apiKey = strings.TrimSpace(apiKey)
	if pepper == "" {
		return "", fmt.Errorf("api key pepper is required")
	}
	if apiKey == "" {
		return "", fmt.Errorf("api key is required")
	}

	mac := hmac.New(sha256.New, []byte(pepper))
	_, _ = mac.Write([]byte(apiKey))
	return hex.EncodeToString(mac.Sum(nil)), nil
}

// HashAdminToken returns sha256(token) as lowercase hex.
func HashAdminToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
