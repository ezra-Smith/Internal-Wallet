package logic

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeWithdrawAuditWhitelistWalletNameRequired(t *testing.T) {
	t.Run("required", func(t *testing.T) {
		_, err := normalizeWithdrawAuditWhitelistWalletNameRequired("   ")
		require.Error(t, err)
		require.Contains(t, err.Error(), "required")
	})

	t.Run("max runes", func(t *testing.T) {
		long := strings.Repeat("a", withdrawAuditWhitelistWalletNameMaxRunes+1)
		_, err := normalizeWithdrawAuditWhitelistWalletNameRequired(long)
		require.Error(t, err)
		require.Contains(t, err.Error(), "too long")
	})

	t.Run("ok", func(t *testing.T) {
		got, err := normalizeWithdrawAuditWhitelistWalletNameRequired("  My Wallet  ")
		require.NoError(t, err)
		require.Equal(t, "My Wallet", got)
	})
}

func TestNormalizeWithdrawAuditWhitelistWalletIcon_RawBase64(t *testing.T) {
	t.Run("png", func(t *testing.T) {
		pngSig := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
		raw := base64.StdEncoding.EncodeToString(pngSig)
		got, err := normalizeWithdrawAuditWhitelistWalletIcon(raw)
		require.NoError(t, err)
		require.Equal(t, "data:image/png;base64,"+base64.StdEncoding.EncodeToString(pngSig), got)
	})

	t.Run("jpeg", func(t *testing.T) {
		jpegSig := []byte{0xFF, 0xD8, 0xFF, 0x00}
		raw := base64.StdEncoding.EncodeToString(jpegSig)
		got, err := normalizeWithdrawAuditWhitelistWalletIcon(raw)
		require.NoError(t, err)
		require.Equal(t, "data:image/jpeg;base64,"+base64.StdEncoding.EncodeToString(jpegSig), got)
	})

	t.Run("webp", func(t *testing.T) {
		webpSig := []byte{'R', 'I', 'F', 'F', 0, 0, 0, 0, 'W', 'E', 'B', 'P'}
		raw := base64.StdEncoding.EncodeToString(webpSig)
		got, err := normalizeWithdrawAuditWhitelistWalletIcon(raw)
		require.NoError(t, err)
		require.Equal(t, "data:image/webp;base64,"+base64.StdEncoding.EncodeToString(webpSig), got)
	})
}

func TestNormalizeWithdrawAuditWhitelistWalletIcon_DataURI(t *testing.T) {
	pngSig := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	payload := base64.StdEncoding.EncodeToString(pngSig)

	t.Run("ok", func(t *testing.T) {
		got, err := normalizeWithdrawAuditWhitelistWalletIcon("data:image/png;base64," + payload)
		require.NoError(t, err)
		require.Equal(t, "data:image/png;base64,"+payload, got)
	})

	t.Run("whitespace payload ok", func(t *testing.T) {
		got, err := normalizeWithdrawAuditWhitelistWalletIcon("data:image/png;base64,\n" + payload + "\n")
		require.NoError(t, err)
		require.Equal(t, "data:image/png;base64,"+payload, got)
	})

	t.Run("mime mismatch", func(t *testing.T) {
		jpegSig := []byte{0xFF, 0xD8, 0xFF, 0x00}
		jpegPayload := base64.StdEncoding.EncodeToString(jpegSig)
		got, err := normalizeWithdrawAuditWhitelistWalletIcon("data:image/png;base64," + jpegPayload)
		require.NoError(t, err)
		require.Equal(t, "", got)
	})

	t.Run("unsupported mime", func(t *testing.T) {
		got, err := normalizeWithdrawAuditWhitelistWalletIcon("data:image/gif;base64," + payload)
		require.NoError(t, err)
		require.Equal(t, "", got)
	})
}

func TestNormalizeWithdrawAuditWhitelistWalletIcon_InvalidOrTooLarge(t *testing.T) {
	t.Run("invalid base64", func(t *testing.T) {
		got, err := normalizeWithdrawAuditWhitelistWalletIcon("not_base64")
		require.NoError(t, err)
		require.Equal(t, "", got)
	})

	t.Run("too large chars", func(t *testing.T) {
		tooLarge := strings.Repeat("A", withdrawAuditWhitelistWalletIconMaxChars+1)
		got, err := normalizeWithdrawAuditWhitelistWalletIcon(tooLarge)
		require.NoError(t, err)
		require.Equal(t, "", got)
	})

	t.Run("too large decoded bytes", func(t *testing.T) {
		large := make([]byte, withdrawAuditWhitelistWalletIconMaxBytes+1)
		copy(large, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}) // PNG signature
		raw := base64.StdEncoding.EncodeToString(large)
		got, err := normalizeWithdrawAuditWhitelistWalletIcon(raw)
		require.NoError(t, err)
		require.Equal(t, "", got)
	})
}
