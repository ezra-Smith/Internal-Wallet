package security

import (
	"encoding/base32"
	"strings"
	"testing"
	"time"
)

func TestEncryptDecryptSecretForStorage_RoundTrip(t *testing.T) {
	t.Parallel()

	plain := "JBSWY3DPEHPK3PXP"
	keyMaterial := "test-key-material"

	enc, err := EncryptSecretForStorage(plain, keyMaterial)
	if err != nil {
		t.Fatalf("EncryptSecretForStorage error: %v", err)
	}
	if !strings.HasPrefix(enc, twoFASecretEncPrefix) {
		t.Fatalf("expected prefix %q, got %q", twoFASecretEncPrefix, enc)
	}

	dec, err := DecryptSecretFromStorage(enc, keyMaterial)
	if err != nil {
		t.Fatalf("DecryptSecretFromStorage error: %v", err)
	}
	if dec != plain {
		t.Fatalf("decrypt mismatch: got %q want %q", dec, plain)
	}
}

func TestDecryptSecretFromStorage_PlaintextCompatibility(t *testing.T) {
	t.Parallel()

	plain := "JBSWY3DPEHPK3PXP"
	got, err := DecryptSecretFromStorage(plain, "ignored")
	if err != nil {
		t.Fatalf("DecryptSecretFromStorage error: %v", err)
	}
	if got != plain {
		t.Fatalf("got %q want %q", got, plain)
	}
}

func TestDecryptSecretFromStorage_WrongKey(t *testing.T) {
	t.Parallel()

	plain := "JBSWY3DPEHPK3PXP"
	enc, err := EncryptSecretForStorage(plain, "k1")
	if err != nil {
		t.Fatalf("EncryptSecretForStorage error: %v", err)
	}
	if _, err := DecryptSecretFromStorage(enc, "k2"); err == nil {
		t.Fatalf("expected decrypt error with wrong key")
	}
}

func TestVerifyTOTP_KnownTime(t *testing.T) {
	t.Parallel()

	// Use a deterministic 20-byte secret.
	raw := []byte("12345678901234567890")
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)

	// RFC 6238 test vectors use SHA1 and 30s step; at time 59s, code is 94287082 for 8 digits.
	// Our implementation uses 6 digits, so compare against the 6-digit truncation (287082).
	at := time.Unix(59, 0).UTC()
	ok, err := VerifyTOTP(secret, "287082", at, 0)
	if err != nil {
		t.Fatalf("VerifyTOTP error: %v", err)
	}
	if !ok {
		t.Fatalf("expected VerifyTOTP to succeed")
	}
}
