package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net"
	"net/http"
	"strings"
	"sync"

	"internalwallet/common/httpx"
)

// authErrorMessages 认证相关错误消息的国际化翻译
// 由于导入循环限制，在 middleware 包内直接维护翻译表
var authErrorMessages = map[string]map[string]string{
	"AUTH_TOKEN_EXPIRED": {
		"zh-CN": "登录已过期",
		"en-US": "Token expired",
	},
	"AUTH_TOKEN_INVALID": {
		"zh-CN": "未授权",
		"en-US": "Unauthorized",
	},
	"AUTH_TOKEN_REVOKED": {
		"zh-CN": "登录已失效",
		"en-US": "Token revoked",
	},
}

// translateAuthError 翻译认证错误消息
func translateAuthError(ctx context.Context, key string) string {
	translations, ok := authErrorMessages[key]
	if !ok {
		return key
	}

	// 从 context 获取语言偏好
	locale := GetLocale(ctx)
	locale = strings.TrimSpace(locale)

	// 默认使用中文
	if locale == "" {
		locale = "zh-CN"
	}

	// 尝试完全匹配
	if msg, ok := translations[locale]; ok {
		return msg
	}

	// 尝试语言前缀匹配（如 "zh" 匹配 "zh-CN"）
	if len(locale) >= 2 {
		prefix := strings.ToLower(locale[:2])
		for k, msg := range translations {
			if strings.HasPrefix(strings.ToLower(k), prefix) {
				return msg
			}
		}
	}

	// 默认返回中文
	if msg, ok := translations["zh-CN"]; ok {
		return msg
	}

	return key
}

// writeJSONError 写入 JSON 错误响应
func writeJSONError(w http.ResponseWriter, message string, statusCode int) {
	writeJSONErrorWithContext(context.Background(), w, message, statusCode)
}

// writeJSONErrorWithContext 写入带国际化支持的 JSON 错误响应
// 如果 message 是已知的 i18n key（如 AUTH_TOKEN_EXPIRED），会根据请求语言返回本地化消息
func writeJSONErrorWithContext(ctx context.Context, w http.ResponseWriter, message string, statusCode int) {
	requestID := w.Header().Get(httpx.RequestIDHeader)
	if strings.TrimSpace(requestID) == "" {
		requestID = generateRequestID()
		w.Header().Set(httpx.RequestIDHeader, requestID)
	}

	// 尝试翻译认证相关错误消息
	localizedMsg := translateAuthError(ctx, message)

	resp := httpx.UnifiedResponse{
		Success:   false,
		Code:      httpx.BizCodeFromHTTPStatus(statusCode),
		Message:   localizedMsg,
		RequestID: requestID,
	}
	_ = httpx.Write(w, statusCode, resp)
}

// Chain 中间件链
func Chain(middlewares ...func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return next
	}
}

// RequestID 请求ID中间件
func RequestID() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := r.Header.Get(httpx.RequestIDHeader)
			if requestID == "" {
				requestID = generateRequestID()
			}

			// 设置响应头
			w.Header().Set(httpx.RequestIDHeader, requestID)

			// 存入 Context，供后续中间件/拦截器透传到 gRPC Metadata
			ctx := context.WithValue(r.Context(), RequestIDKey, requestID)

			// 继续处理
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// generateRequestID 生成请求ID（简单实现）
func generateRequestID() string {
	// 使用 crypto/rand，避免可预测的 request id
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// 极端情况下的兜底（仍需保证不会 panic）
		return "req-unknown"
	}
	return "req-" + hex.EncodeToString(b)
}

// Timeout 超时中间件
func Timeout(maxDuration int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 简单实现，生产环境可以使用 context.WithTimeout
			next.ServeHTTP(w, r)
		})
	}
}

// TrustedProxyConfig 可信代理配置
type TrustedProxyConfig struct {
	mu               sync.RWMutex
	trustedCIDRs     []*net.IPNet
	enableProxyTrust bool
}

var (
	// globalProxyConfig 全局可信代理配置（可通过环境变量或配置文件初始化）
	globalProxyConfig = &TrustedProxyConfig{
		enableProxyTrust: false, // 默认不信任代理头（安全优先）
		trustedCIDRs:     make([]*net.IPNet, 0),
	}
)

// SetTrustedProxies 设置可信代理CIDR列表
// 示例: SetTrustedProxies([]string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "127.0.0.1/32"})
func SetTrustedProxies(cidrs []string) error {
	trustedNets := make([]*net.IPNet, 0, len(cidrs))

	for _, cidr := range cidrs {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			return err
		}
		trustedNets = append(trustedNets, ipNet)
	}

	globalProxyConfig.mu.Lock()
	defer globalProxyConfig.mu.Unlock()

	globalProxyConfig.trustedCIDRs = trustedNets
	globalProxyConfig.enableProxyTrust = len(trustedNets) > 0

	return nil
}

// isTrustedProxy 检查给定的地址是否是可信代理
func isTrustedProxy(remoteAddr string) bool {
	globalProxyConfig.mu.RLock()
	defer globalProxyConfig.mu.RUnlock()

	// 如果未启用代理信任，直接返回 false
	if !globalProxyConfig.enableProxyTrust {
		return false
	}

	// 解析远程地址（可能包含端口）
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		// 如果没有端口，直接使用原地址
		host = remoteAddr
	}

	// 解析IP地址
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}

	// 检查IP是否在可信CIDR列表中
	for _, trustedNet := range globalProxyConfig.trustedCIDRs {
		if trustedNet.Contains(ip) {
			return true
		}
	}

	return false
}

// isValidIP 验证字符串是否为有效的IP地址
func isValidIP(ipStr string) bool {
	return net.ParseIP(ipStr) != nil
}

// getClientIP 获取客户端真实 IP（安全版本）
// 仅当请求来自可信代理时才信任 X-Forwarded-For/X-Real-IP 标头
func getClientIP(r *http.Request) string {
	remoteAddr := r.RemoteAddr

	// 检查是否来自可信代理
	if isTrustedProxy(remoteAddr) {
		// 优先处理 X-Forwarded-For（标准代理头）
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			// X-Forwarded-For 格式: client, proxy1, proxy2
			// 取第一个IP（最左边的是真实客户端IP）
			ips := strings.Split(forwarded, ",")
			if len(ips) > 0 {
				clientIP := strings.TrimSpace(ips[0])
				// 验证IP有效性
				if isValidIP(clientIP) {
					return clientIP
				}
			}
		}

		// 其次处理 X-Real-IP
		if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
			realIP = strings.TrimSpace(realIP)
			// 验证IP有效性
			if isValidIP(realIP) {
				return realIP
			}
		}
	}

	// 不是可信代理或没有有效的代理头，使用 RemoteAddr
	// 移除端口号（如果有）
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		// 没有端口，直接返回
		return remoteAddr
	}

	return host
}
