package middleware

import (
	"net/http"
	"strconv"
	"strings"
)

// CORSConfig CORS 配置
type CORSConfig struct {
	AllowOrigins     []string
	AllowMethods     []string
	AllowHeaders     []string
	ExposeHeaders    []string
	AllowCredentials bool
	MaxAge           int
}

// DefaultCORSConfig 默认 CORS 配置
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodDelete,
			http.MethodPatch,
			http.MethodOptions,
		},
		AllowHeaders: []string{
			"Content-Type",
			"Authorization",
			"Idempotency-Key",
			"X-Request-ID",
			"X-Trace-ID",
		},
		ExposeHeaders: []string{
			"X-Request-ID",
			"X-Trace-ID",
		},
		AllowCredentials: true,
		MaxAge:           86400, // 24小时
	}
}

// CORS CORS 中间件
func CORS(config CORSConfig) func(http.Handler) http.Handler {
	// 安全检查：禁止 "*" + AllowCredentials: true 的危险组合
	if config.AllowCredentials && hasWildcard(config.AllowOrigins) {
		panic("SECURITY: CORS configuration error - Cannot use AllowOrigins='*' with AllowCredentials=true. " +
			"This is a security vulnerability. Please specify exact allowed origins.")
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// 检查是否允许该 origin
			if isOriginAllowed(origin, config.AllowOrigins) {
				// 如果配置了通配符且不使用凭证，可以返回 "*"
				if hasWildcard(config.AllowOrigins) && !config.AllowCredentials {
					w.Header().Set("Access-Control-Allow-Origin", "*")
				} else {
					// 否则返回具体的 origin（更安全）
					w.Header().Set("Access-Control-Allow-Origin", origin)
				}
			}

			// 设置其他 CORS 头
			if config.AllowCredentials {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			if len(config.ExposeHeaders) > 0 {
				w.Header().Set("Access-Control-Expose-Headers", strings.Join(config.ExposeHeaders, ", "))
			}

			// 处理 OPTIONS 预检请求
			if r.Method == http.MethodOptions {
				if len(config.AllowMethods) > 0 {
					w.Header().Set("Access-Control-Allow-Methods", strings.Join(config.AllowMethods, ", "))
				}

				if len(config.AllowHeaders) > 0 {
					w.Header().Set("Access-Control-Allow-Headers", strings.Join(config.AllowHeaders, ", "))
				}

				if config.MaxAge > 0 {
					w.Header().Set("Access-Control-Max-Age", strconv.Itoa(config.MaxAge))
				}

				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// isOriginAllowed 检查 origin 是否在允许列表中
func isOriginAllowed(origin string, allowOrigins []string) bool {
	if len(allowOrigins) == 0 {
		return false
	}

	for _, allowed := range allowOrigins {
		if allowed == "*" {
			return true
		}
		if allowed == origin {
			return true
		}
		// 支持通配符匹配（简单实现）
		if strings.HasSuffix(allowed, "*") {
			prefix := strings.TrimSuffix(allowed, "*")
			if strings.HasPrefix(origin, prefix) {
				return true
			}
		}
	}

	return false
}

// hasWildcard 检查是否包含通配符
func hasWildcard(allowOrigins []string) bool {
	for _, origin := range allowOrigins {
		if origin == "*" || strings.Contains(origin, "*") {
			return true
		}
	}
	return false
}
