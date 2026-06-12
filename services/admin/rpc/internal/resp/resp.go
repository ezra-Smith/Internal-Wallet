package resp

import (
	"context"
	"strconv"
	"strings"
	"time"

	"internalwallet/common/i18n"
	"internalwallet/common/middleware"
)

func Timestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func RequestID(ctx context.Context) string {
	return middleware.GetRequestID(ctx)
}

// Msg returns a localized message by key for the current request.
func Msg(ctx context.Context, key string) string {
	return i18n.T(ctx, key, nil)
}

// MsgWithData returns a localized message by key, rendered with optional template data.
func MsgWithData(ctx context.Context, key string, data map[string]interface{}) string {
	return i18n.T(ctx, key, data)
}

// AdminIDString 返回 SRD 约定的 admin-<id> 展示形式
func AdminIDString(id int64) string {
	return "admin-" + strconv.FormatInt(id, 10)
}

// ParseAdminID 支持 "admin-123" / "123"
func ParseAdminID(s string) (int64, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "admin-")
	return strconv.ParseInt(s, 10, 64)
}
