package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

const twoFASecretEncPrefix = "enc:"

// EncryptSecretForStorage encrypts a TOTP secret for DB storage using AES-256-GCM.
// The output format is: "enc:" + base64url(nonce||ciphertext).
func EncryptSecretForStorage(secretPlain string, keyMaterial string) (string, error) {
	secretPlain = strings.TrimSpace(secretPlain)
	if secretPlain == "" {
		return "", fmt.Errorf("empty secret")
	}
	keyMaterial = strings.TrimSpace(keyMaterial)
	if keyMaterial == "" {
		return "", fmt.Errorf("empty key material")
	}

	key := sha256.Sum256([]byte(keyMaterial))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nil, nonce, []byte(secretPlain), nil)
	out := make([]byte, 0, len(nonce)+len(ciphertext))
	out = append(out, nonce...)
	out = append(out, ciphertext...)
	return twoFASecretEncPrefix + base64.RawURLEncoding.EncodeToString(out), nil
}

// DecryptSecretFromStorage decrypts a TOTP secret read from DB.
// Backward-compatible: if the secret doesn't have the "enc:" prefix, it is returned as-is.
func DecryptSecretFromStorage(secretStored string, keyMaterial string) (string, error) {
	secretStored = strings.TrimSpace(secretStored)
	if secretStored == "" {
		return "", nil
	}
	if !strings.HasPrefix(secretStored, twoFASecretEncPrefix) {
		return secretStored, nil
	}
	keyMaterial = strings.TrimSpace(keyMaterial)
	if keyMaterial == "" {
		return "", fmt.Errorf("empty key material")
	}

	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(secretStored, twoFASecretEncPrefix))
	if err != nil {
		return "", fmt.Errorf("invalid encrypted secret")
	}

	key := sha256.Sum256([]byte(keyMaterial))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", fmt.Errorf("invalid encrypted secret")
	}

	nonce := raw[:gcm.NonceSize()]
	ciphertext := raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("invalid encrypted secret")
	}
	return string(plain), nil
}
