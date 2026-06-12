package middleware

import (
	"context"
	"net/http"
	"strings"
)

// MetadataKey 用于在 Context 中存储元数据的键类型
type MetadataKey string

const (
	// ClientIPKey 客户端IP
	ClientIPKey MetadataKey = "client-ip"
	// UserAgentKey User-Agent
	UserAgentKey MetadataKey = "user-agent"
	// DeviceFingerprintKey 设备指纹
	DeviceFingerprintKey MetadataKey = "device-fingerprint"
	// DeviceIDKey 设备ID
	DeviceIDKey MetadataKey = "device-id"
	// PlatformKey 平台（iOS/Android/Web）
	PlatformKey MetadataKey = "platform"
	// AppVersionKey APP版本
	AppVersionKey MetadataKey = "app-version"
	// LocaleKey 用户语言偏好（X-Lang / Accept-Language）
	LocaleKey MetadataKey = "locale"

	// HTTPMethodKey HTTP 方法（由网关注入）
	HTTPMethodKey MetadataKey = "http-method"
	// HTTPPathKey HTTP 路径（由网关注入）
	HTTPPathKey MetadataKey = "http-path"

	// RequestIDKey 请求ID（由 RequestID 中间件注入）
	RequestIDKey MetadataKey = "request-id"
	// IdempotencyKeyKey 幂等键（由网关注入/透传）
	IdempotencyKeyKey MetadataKey = "idempotency-key"

	// JWTIDKey JWT jti
	JWTIDKey MetadataKey = "jwt-id"
	// JWTExpiresAtKey JWT exp（Unix秒）
	JWTExpiresAtKey MetadataKey = "jwt-exp"
	// JWTTokenVersionKey JWT token version
	JWTTokenVersionKey MetadataKey = "jwt-ver"
)

// ClientMetadata 客户端元数据
type ClientMetadata struct {
	IP                string
	UserAgent         string
	DeviceFingerprint string
	DeviceID          string
	Platform          string
	AppVersion        string
}

// ExtractClientMetadata 提取客户端元数据中间件
// 从HTTP请求头中提取IP、User-Agent、设备指纹等信息，并存储到 Context 中
func ExtractClientMetadata() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// 1. 提取客户端IP
			clientIP := extractClientIP(r)
			ctx = context.WithValue(ctx, ClientIPKey, clientIP)

			// 2. 提取 User-Agent
			userAgent := r.Header.Get("User-Agent")
			if userAgent != "" {
				ctx = context.WithValue(ctx, UserAgentKey, userAgent)
			}

			// 3. 提取设备指纹（自定义Header）
			deviceFingerprint := r.Header.Get("X-Device-Fingerprint")
			if deviceFingerprint != "" {
				ctx = context.WithValue(ctx, DeviceFingerprintKey, deviceFingerprint)
			}

			// 4. 提取设备ID（自定义Header）
			deviceID := r.Header.Get("X-Device-ID")
			if deviceID != "" {
				ctx = context.WithValue(ctx, DeviceIDKey, deviceID)
			}

			// 5. 提取平台信息（自定义Header）
			platform := r.Header.Get("X-Platform")
			if platform != "" {
				ctx = context.WithValue(ctx, PlatformKey, platform)
			}

			// 6. 提取APP版本（自定义Header）
			appVersion := r.Header.Get("X-App-Version")
			if appVersion != "" {
				ctx = context.WithValue(ctx, AppVersionKey, appVersion)
			}

			// 6.1 提取语言偏好（优先 X-Lang，其次 Accept-Language）
			locale := strings.TrimSpace(r.Header.Get("X-Lang"))
			if locale == "" {
				locale = strings.TrimSpace(r.Header.Get("Accept-Language"))
			}
			if locale != "" {
				ctx = context.WithValue(ctx, LocaleKey, locale)
			}

			// 7. 幂等键（标准 Header）
			idempotencyKey := r.Header.Get("Idempotency-Key")
			if idempotencyKey != "" {
				ctx = context.WithValue(ctx, IdempotencyKeyKey, idempotencyKey)
			}

			// 继续处理，使用新的 context
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// extractClientIP 提取客户端真实IP
// 优先级：X-Forwarded-For > X-Real-IP > RemoteAddr
func extractClientIP(r *http.Request) string {
	// 1. 尝试从 X-Forwarded-For 获取（可能包含多个IP，取第一个）
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		// X-Forwarded-For 格式: client, proxy1, proxy2
		ips := strings.Split(forwarded, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// 2. 尝试从 X-Real-IP 获取
	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	// 3. 从 RemoteAddr 获取
	// RemoteAddr 格式可能是 "IP:Port" 或 "IP"
	remoteAddr := r.RemoteAddr
	if idx := strings.LastIndex(remoteAddr, ":"); idx != -1 {
		return remoteAddr[:idx]
	}

	return remoteAddr
}

// GetClientIP 从 Context 获取客户端IP
func GetClientIP(ctx context.Context) string {
	if ip, ok := ctx.Value(ClientIPKey).(string); ok {
		return ip
	}
	return ""
}

// GetUserAgent 从 Context 获取 User-Agent
func GetUserAgent(ctx context.Context) string {
	if ua, ok := ctx.Value(UserAgentKey).(string); ok {
		return ua
	}
	return ""
}

// GetDeviceFingerprint 从 Context 获取设备指纹
func GetDeviceFingerprint(ctx context.Context) string {
	if fp, ok := ctx.Value(DeviceFingerprintKey).(string); ok {
		return fp
	}
	return ""
}

// GetDeviceID 从 Context 获取设备ID
func GetDeviceID(ctx context.Context) string {
	if id, ok := ctx.Value(DeviceIDKey).(string); ok {
		return id
	}
	return ""
}

// GetPlatform 从 Context 获取平台
func GetPlatform(ctx context.Context) string {
	if platform, ok := ctx.Value(PlatformKey).(string); ok {
		return platform
	}
	return ""
}

// GetAppVersion 从 Context 获取APP版本
func GetAppVersion(ctx context.Context) string {
	if version, ok := ctx.Value(AppVersionKey).(string); ok {
		return version
	}
	return ""
}

// GetLocale 从 Context 获取语言偏好（X-Lang / Accept-Language）
func GetLocale(ctx context.Context) string {
	if locale, ok := ctx.Value(LocaleKey).(string); ok {
		return locale
	}
	return ""
}

// GetHTTPMethod 从 Context 获取 HTTP Method
func GetHTTPMethod(ctx context.Context) string {
	if method, ok := ctx.Value(HTTPMethodKey).(string); ok {
		return strings.TrimSpace(method)
	}
	return ""
}

// GetHTTPPath 从 Context 获取 HTTP Path
func GetHTTPPath(ctx context.Context) string {
	if path, ok := ctx.Value(HTTPPathKey).(string); ok {
		return strings.TrimSpace(path)
	}
	return ""
}

// GetClientMetadata 从 Context 获取所有客户端元数据
func GetClientMetadata(ctx context.Context) *ClientMetadata {
	return &ClientMetadata{
		IP:                GetClientIP(ctx),
		UserAgent:         GetUserAgent(ctx),
		DeviceFingerprint: GetDeviceFingerprint(ctx),
		DeviceID:          GetDeviceID(ctx),
		Platform:          GetPlatform(ctx),
		AppVersion:        GetAppVersion(ctx),
	}
}

// GetRequestID 从 Context 获取请求ID
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(RequestIDKey).(string); ok {
		return id
	}
	return ""
}

// GetIdempotencyKey 从 Context 获取幂等键
func GetIdempotencyKey(ctx context.Context) string {
	if key, ok := ctx.Value(IdempotencyKeyKey).(string); ok {
		return key
	}
	return ""
}

// GetJWTID 从 Context 获取 JWT jti
func GetJWTID(ctx context.Context) string {
	if id, ok := ctx.Value(JWTIDKey).(string); ok {
		return id
	}
	return ""
}

// GetJWTExpiresAt 从 Context 获取 JWT exp（Unix秒）
func GetJWTExpiresAt(ctx context.Context) int64 {
	if v, ok := ctx.Value(JWTExpiresAtKey).(int64); ok {
		return v
	}
	return 0
}

// GetJWTTokenVersion 从 Context 获取 JWT token version
func GetJWTTokenVersion(ctx context.Context) int64 {
	if v, ok := ctx.Value(JWTTokenVersionKey).(int64); ok {
		return v
	}
	return 0
}
