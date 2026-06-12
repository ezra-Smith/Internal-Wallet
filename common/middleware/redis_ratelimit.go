package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
)

// RedisRateLimiter 基于 Redis 的分布式限流器
// 使用滑动窗口算法，支持集群部署
type RedisRateLimiter struct {
	client    *redis.Client
	rate      int           // 每秒允许的请求数
	window    time.Duration // 时间窗口（默认1秒）
	keyPrefix string        // Redis键前缀
}

// NewRedisRateLimiter 创建基于 Redis 的限流器
func NewRedisRateLimiter(client *redis.Client, reqPerSec int) *RedisRateLimiter {
	return &RedisRateLimiter{
		client:    client,
		rate:      reqPerSec,
		window:    time.Second,
		keyPrefix: "gateway:ratelimit",
	}
}

// Allow 检查是否允许请求通过（带路由和IP信息）
func (r *RedisRateLimiter) Allow(ctx context.Context, routePath string, ip string) (bool, error) {
	// 构建Redis键：gateway:ratelimit:{route}:{ip}
	key := fmt.Sprintf("%s:%s:%s", r.keyPrefix, routePath, ip)

	now := time.Now()
	windowStart := now.Add(-r.window)

	// 使用Lua脚本保证原子性
	script := redis.NewScript(`
		-- KEYS[1]: Redis键
		-- ARGV[1]: 窗口起始时间（毫秒时间戳）
		-- ARGV[2]: 当前时间（毫秒时间戳）
		-- ARGV[3]: 限流配额
		-- ARGV[4]: 过期时间（秒）

		-- 1. 移除窗口外的旧记录
		redis.call('ZREMRANGEBYSCORE', KEYS[1], 0, ARGV[1])

		-- 2. 统计窗口内的请求数
		local count = redis.call('ZCARD', KEYS[1])

		-- 3. 检查是否超限
		if count < tonumber(ARGV[3]) then
			-- 4. 未超限，添加当前请求
			redis.call('ZADD', KEYS[1], ARGV[2], ARGV[2])
			-- 5. 设置过期时间（防止内存泄漏）
			redis.call('EXPIRE', KEYS[1], ARGV[4])
			return 1
		else
			return 0
		end
	`)

	result, err := script.Run(
		ctx,
		r.client,
		[]string{key},
		windowStart.UnixMilli(),   // ARGV[1]
		now.UnixMilli(),           // ARGV[2]
		r.rate,                    // ARGV[3]
		int(r.window.Seconds())*2, // ARGV[4] - 过期时间设为窗口的2倍
	).Result()

	if err != nil {
		logx.Errorf("Redis rate limit error: %v", err)
		return false, err
	}

	// result == 1 表示允许通过
	return result.(int64) == 1, nil
}

// Middleware 返回HTTP中间件（IP限流模式）
func (r *RedisRateLimiter) Middleware(routePath string) func(http.Handler) http.Handler {
	return r.MiddlewareWithType(routePath, "ip")
}

// MiddlewareWithType 返回HTTP中间件（支持指定限流类型）
// limitType: "ip" 或 "user"
func (r *RedisRateLimiter) MiddlewareWithType(routePath string, limitType string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			var identifier string
			var identifierType string

			switch limitType {
			case "user":
				// 用户限流：从Context中获取UserID
				userID := GetUserID(req.Context())
				if userID == "" {
					// 没有UserID，记录警告并放行（可能是JWT中间件未执行）
					logx.Infof("User rate limit: no user ID found for %s, request allowed", routePath)
					next.ServeHTTP(w, req)
					return
				}
				identifier = userID
				identifierType = "user"

			case "ip":
				// IP限流：获取客户端IP
				identifier = getClientIP(req)
				identifierType = "IP"

			default:
				// 未知类型，记录错误并放行
				logx.Errorf("Unknown rate limit type: %s, request allowed", limitType)
				next.ServeHTTP(w, req)
				return
			}

			// 检查限流
			allowed, err := r.Allow(req.Context(), routePath, identifier)
			if err != nil {
				// Redis错误，记录日志但放行请求（降级策略）
				logx.Errorf("Redis rate limit check failed for %s from %s %s: %v", routePath, identifierType, identifier, err)
				next.ServeHTTP(w, req)
				return
			}

			if !allowed {
				// 超过限流配额
				writeJSONError(w, fmt.Sprintf("Rate limit exceeded for %s: %s", identifierType, identifier), http.StatusTooManyRequests)
				return
			}

			// 通过限流检查
			next.ServeHTTP(w, req)
		})
	}
}

// GetStats 获取限流统计信息（可选功能）
func (r *RedisRateLimiter) GetStats(ctx context.Context, routePath string, ip string) (int64, error) {
	key := fmt.Sprintf("%s:%s:%s", r.keyPrefix, routePath, ip)
	windowStart := time.Now().Add(-r.window)

	// 清理过期记录
	r.client.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", windowStart.UnixMilli()))

	// 返回当前窗口内的请求数
	return r.client.ZCard(ctx, key).Result()
}

// CleanupExpired 清理过期的限流记录（可选的定期任务）
func (r *RedisRateLimiter) CleanupExpired(ctx context.Context, routePath string, ip string) error {
	key := fmt.Sprintf("%s:%s:%s", r.keyPrefix, routePath, ip)
	windowStart := time.Now().Add(-r.window)

	return r.client.ZRemRangeByScore(
		ctx,
		key,
		"0",
		fmt.Sprintf("%d", windowStart.UnixMilli()),
	).Err()
}
