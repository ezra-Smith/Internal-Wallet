package middleware

import (
	"net/http"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
)

// responseWriter 包装 http.ResponseWriter 以捕获状态码和响应大小
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	size, err := rw.ResponseWriter.Write(b)
	rw.size += size
	return size, err
}

// Logging 日志中间件
func Logging() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// 包装 ResponseWriter
			rw := newResponseWriter(w)

			// 处理请求
			next.ServeHTTP(rw, r)

			// 记录日志
			duration := time.Since(start)

			logx.WithContext(r.Context()).Infow("http request",
				logx.Field("method", r.Method),
				logx.Field("path", r.URL.Path),
				logx.Field("query", r.URL.RawQuery),
				logx.Field("status", rw.statusCode),
				logx.Field("duration", duration.Milliseconds()),
				logx.Field("size", rw.size),
				logx.Field("ip", getClientIP(r)),
				logx.Field("user_agent", r.UserAgent()),
			)
		})
	}
}

// DetailedLogging 详细日志中间件（包含请求头、请求体等）
func DetailedLogging() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// 包装 ResponseWriter
			rw := newResponseWriter(w)

			// 获取用户信息（如果有）
			userID := GetUserID(r.Context())

			// 记录请求开始
			logx.WithContext(r.Context()).Infow("request started",
				logx.Field("method", r.Method),
				logx.Field("path", r.URL.Path),
				logx.Field("query", r.URL.RawQuery),
				logx.Field("ip", getClientIP(r)),
				logx.Field("user_id", userID),
				logx.Field("user_agent", r.UserAgent()),
				logx.Field("referer", r.Referer()),
			)

			// 处理请求
			next.ServeHTTP(rw, r)

			// 记录请求完成
			duration := time.Since(start)

			logx.WithContext(r.Context()).Infow("request completed",
				logx.Field("method", r.Method),
				logx.Field("path", r.URL.Path),
				logx.Field("status", rw.statusCode),
				logx.Field("duration_ms", duration.Milliseconds()),
				logx.Field("response_size", rw.size),
				logx.Field("user_id", userID),
			)
		})
	}
}

// AccessLog 访问日志中间件（简单版）
func AccessLog() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := newResponseWriter(w)

			next.ServeHTTP(rw, r)

			duration := time.Since(start)

			// 简单格式：类似 Nginx access log
			logx.Infof("%s - - [%s] \"%s %s %s\" %d %d \"%s\" \"%s\" %dms",
				getClientIP(r),
				start.Format("02/Jan/2006:15:04:05 -0700"),
				r.Method,
				r.URL.Path,
				r.Proto,
				rw.statusCode,
				rw.size,
				r.Referer(),
				r.UserAgent(),
				duration.Milliseconds(),
			)
		})
	}
}
