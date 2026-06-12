package logic

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

func normalizeSeedID(seedID string) string {
	seedID = strings.TrimSpace(seedID)
	if seedID == "" {
		return defaultHotWalletSeedID
	}
	return seedID
}

func splitMnemonicWords(mnemonic string) []string {
	// strings.Fields 会把连续空白合并处理，适合助记词
	return strings.Fields(strings.TrimSpace(mnemonic))
}

func randHex(nBytes int) string {
	if nBytes <= 0 {
		nBytes = 16
	}
	b := make([]byte, nBytes)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func hashWordLower(word string) string {
	w := strings.ToLower(strings.TrimSpace(word))
	sum := sha256.Sum256([]byte(w))
	return hex.EncodeToString(sum[:])
}
