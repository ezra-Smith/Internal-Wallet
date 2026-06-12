package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimit 全局限流中间件（基于令牌桶算法）
func RateLimit(reqPerSec int, burst int) func(http.Handler) http.Handler {
	limiter := rate.NewLimiter(rate.Limit(reqPerSec), burst)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !limiter.Allow() {
				writeJSONError(w, "Too many requests", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// IPRateLimit 基于 IP 的限流中间件
type IPRateLimiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	rate     rate.Limit
	burst    int
	cleanUp  time.Duration
}

// NewIPRateLimiter 创建基于 IP 的限流器
func NewIPRateLimiter(reqPerSec int, burst int, cleanUpInterval time.Duration) *IPRateLimiter {
	limiter := &IPRateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rate:     rate.Limit(reqPerSec),
		burst:    burst,
		cleanUp:  cleanUpInterval,
	}

	// 定期清理不活跃的限流器
	go limiter.cleanupRoutine()

	return limiter
}

// GetLimiter 获取指定 IP 的限流器
func (i *IPRateLimiter) GetLimiter(ip string) *rate.Limiter {
	i.mu.Lock()
	defer i.mu.Unlock()

	limiter, exists := i.limiters[ip]
	if !exists {
		limiter = rate.NewLimiter(i.rate, i.burst)
		i.limiters[ip] = limiter
	}

	return limiter
}

// cleanupRoutine 定期清理不活跃的限流器
func (i *IPRateLimiter) cleanupRoutine() {
	ticker := time.NewTicker(i.cleanUp)
	defer ticker.Stop()

	for range ticker.C {
		i.mu.Lock()
		// 清理所有限流器（简单实现，生产环境可以记录最后访问时间）
		i.limiters = make(map[string]*rate.Limiter)
		i.mu.Unlock()
	}
}

// Middleware 返回中间件函数
func (i *IPRateLimiter) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := getClientIP(r)
			limiter := i.GetLimiter(ip)

			if !limiter.Allow() {
				writeJSONError(w, fmt.Sprintf("Rate limit exceeded for IP: %s", ip), http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// UserRateLimit 基于用户 ID 的限流中间件
type UserRateLimiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	rate     rate.Limit
	burst    int
}

// NewUserRateLimiter 创建基于用户的限流器
func NewUserRateLimiter(reqPerSec int, burst int) *UserRateLimiter {
	return &UserRateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rate:     rate.Limit(reqPerSec),
		burst:    burst,
	}
}

// Middleware 返回中间件函数（需要在 JWT 中间件之后使用）
func (u *UserRateLimiter) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := GetUserID(r.Context())
			if userID == "" {
				// 没有用户信息，跳过限流
				next.ServeHTTP(w, r)
				return
			}

			u.mu.Lock()
			limiter, exists := u.limiters[userID]
			if !exists {
				limiter = rate.NewLimiter(u.rate, u.burst)
				u.limiters[userID] = limiter
			}
			u.mu.Unlock()

			if !limiter.Allow() {
				writeJSONError(w, fmt.Sprintf("Rate limit exceeded for user: %s", userID), http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// parseXForwardedFor 解析 X-Forwarded-For header
func parseXForwardedFor(xff string) []string {
	var ips []string
	for _, ip := range splitAndTrim(xff, ",") {
		if ip != "" {
			ips = append(ips, ip)
		}
	}
	return ips
}

// splitAndTrim 分割并去除空格
func splitAndTrim(s, sep string) []string {
	parts := make([]string, 0)
	for _, part := range splitString(s, sep) {
		trimmed := trimSpace(part)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}

// splitString 简单的字符串分割
func splitString(s, sep string) []string {
	if s == "" {
		return []string{}
	}
	// 简单实现，生产环境可以用 strings.Split
	result := []string{}
	current := ""
	for i := 0; i < len(s); i++ {
		if string(s[i]) == sep {
			result = append(result, current)
			current = ""
		} else {
			current += string(s[i])
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

// trimSpace 去除字符串首尾空格
func trimSpace(s string) string {
	start := 0
	end := len(s)

	for start < end && s[start] == ' ' {
		start++
	}
	for end > start && s[end-1] == ' ' {
		end--
	}

	return s[start:end]
}
