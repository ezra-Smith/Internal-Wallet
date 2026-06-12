package i18n

import (
	"context"
	"encoding/json"
	"testing"

	"internalwallet/common/middleware"

	"github.com/stretchr/testify/require"
)

func TestTranslationKeyParity(t *testing.T) {
	t.Parallel()

	enBytes, err := localeFS.ReadFile("locales/en-US.json")
	require.NoError(t, err)
	zhBytes, err := localeFS.ReadFile("locales/zh-CN.json")
	require.NoError(t, err)

	var en map[string]string
	var zh map[string]string
	require.NoError(t, json.Unmarshal(enBytes, &en))
	require.NoError(t, json.Unmarshal(zhBytes, &zh))

	for key := range en {
		_, ok := zh[key]
		require.Truef(t, ok, "missing zh-CN translation for key: %s", key)
	}
	for key := range zh {
		_, ok := en[key]
		require.Truef(t, ok, "missing en-US translation for key: %s", key)
	}
}

func TestLocaleMatching(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.Background(), middleware.LocaleKey, "en-US,en;q=0.9")
	require.Equal(t, "en-US", Locale(ctx))

	ctx = context.WithValue(context.Background(), middleware.LocaleKey, "en")
	require.Equal(t, "en-US", Locale(ctx))

	ctx = context.WithValue(context.Background(), middleware.LocaleKey, "zh")
	require.Equal(t, "zh-CN", Locale(ctx))
}

func TestTranslateReason(t *testing.T) {
	t.Parallel()

	ctxEn := context.WithValue(context.Background(), middleware.LocaleKey, "en-US")
	require.Equal(t, "Unauthorized", T(ctxEn, "AUTH_TOKEN_INVALID", nil))

	ctxZh := context.WithValue(context.Background(), middleware.LocaleKey, "zh-CN")
	require.Equal(t, "未授权", T(ctxZh, "AUTH_TOKEN_INVALID", nil))

	msg := T(ctxEn, "DB_SCHEMA_MISSING", map[string]interface{}{"table": "admin_menu"})
	require.Contains(t, msg, "admin_menu")
}

func TestValidationTranslation(t *testing.T) {
	t.Parallel()

	ctxEn := context.WithValue(context.Background(), middleware.LocaleKey, "en-US")
	require.Equal(t, "Required", Validation(ctxEn, "required"))

	ctxZh := context.WithValue(context.Background(), middleware.LocaleKey, "zh-CN")
	require.Equal(t, "必填", Validation(ctxZh, "required"))

	// Unknown codes fall back to the original string.
	require.Equal(t, "unknown_code", Validation(ctxEn, "unknown_code"))
}
