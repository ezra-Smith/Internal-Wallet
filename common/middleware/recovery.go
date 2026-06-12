package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/zeromicro/go-zero/core/logx"
)

// Recovery 异常恢复中间件
func Recovery() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					// 记录堆栈信息
					stack := debug.Stack()

					logx.WithContext(r.Context()).Errorw("panic recovered",
						logx.Field("error", err),
						logx.Field("stack", string(stack)),
						logx.Field("method", r.Method),
						logx.Field("path", r.URL.Path),
						logx.Field("ip", getClientIP(r)),
					)

					// 返回 500 错误
					writeJSONError(w, "Internal server error", http.StatusInternalServerError)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// RecoveryWithHandler 带自定义处理器的异常恢复中间件
func RecoveryWithHandler(handler func(http.ResponseWriter, *http.Request, interface{})) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					// 记录堆栈信息
					stack := debug.Stack()

					logx.WithContext(r.Context()).Errorw("panic recovered",
						logx.Field("error", err),
						logx.Field("stack", string(stack)),
						logx.Field("method", r.Method),
						logx.Field("path", r.URL.Path),
					)

					// 调用自定义处理器
					handler(w, r, err)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// DefaultRecoveryHandler 默认的 panic 处理器
func DefaultRecoveryHandler(w http.ResponseWriter, r *http.Request, err interface{}) {
	writeJSONError(w, fmt.Sprintf("Internal server error: %v", err), http.StatusInternalServerError)
}
