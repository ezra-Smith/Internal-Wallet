package logic

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
)

func generateBackupCodes(n int, pepper string, adminID int64) ([]string, []string, error) {
	if n <= 0 {
		n = 10
	}
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	const codeLen = 10

	out := make([]string, 0, n)
	hashes := make([]string, 0, n)
	for i := 0; i < n; i++ {
		code, err := randomStringFromAlphabet(codeLen, alphabet)
		if err != nil {
			return nil, nil, err
		}
		out = append(out, code)
		hashes = append(hashes, hashBackupCode(pepper, adminID, code))
	}
	return out, hashes, nil
}

func randomStringFromAlphabet(length int, alphabet string) (string, error) {
	if length <= 0 {
		return "", fmt.Errorf("invalid length")
	}
	if strings.TrimSpace(alphabet) == "" {
		return "", fmt.Errorf("empty alphabet")
	}
	var sb strings.Builder
	sb.Grow(length)
	max := big.NewInt(int64(len(alphabet)))
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		sb.WriteByte(alphabet[int(n.Int64())])
	}
	return sb.String(), nil
}

func hashBackupCode(pepper string, adminID int64, code string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%s", pepper, adminID, code)))
	return hex.EncodeToString(sum[:])
}
