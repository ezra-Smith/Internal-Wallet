package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// JWTClaims JWT 载荷
type JWTClaims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	// Version 用于支持服务端强制失效（例如禁用账号、踢下线等）
	// Admin 服务会使用该字段；其他服务可忽略。
	Version int64 `json:"ver,omitempty"`
	jwt.RegisteredClaims
}

// JWTAuth JWT 认证中间件
func JWTAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// 从 Header 中获取 token
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeJSONErrorWithContext(ctx, w, "AUTH_TOKEN_INVALID", http.StatusUnauthorized)
				return
			}

			// 检查 Bearer 前缀
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				writeJSONErrorWithContext(ctx, w, "AUTH_TOKEN_INVALID", http.StatusUnauthorized)
				return
			}

			tokenString := parts[1]

			// 解析和验证 token
			token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
				// 验证签名算法
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
				}
				return []byte(secret), nil
			})

			if err != nil {
				// 检查是否是 token 过期错误
				if strings.Contains(err.Error(), "expired") {
					writeJSONErrorWithContext(ctx, w, "AUTH_TOKEN_EXPIRED", http.StatusUnauthorized)
					return
				}
				writeJSONErrorWithContext(ctx, w, "AUTH_TOKEN_INVALID", http.StatusUnauthorized)
				return
			}

			if !token.Valid {
				writeJSONErrorWithContext(ctx, w, "AUTH_TOKEN_INVALID", http.StatusUnauthorized)
				return
			}

			// 获取 claims
			claims, ok := token.Claims.(*JWTClaims)
			if !ok {
				writeJSONErrorWithContext(ctx, w, "AUTH_TOKEN_INVALID", http.StatusUnauthorized)
				return
			}

			// 检查过期时间
			if claims.ExpiresAt != nil && claims.ExpiresAt.Time.Before(time.Now()) {
				writeJSONErrorWithContext(ctx, w, "AUTH_TOKEN_EXPIRED", http.StatusUnauthorized)
				return
			}

			// 将用户信息存入 context
			ctx = context.WithValue(ctx, "user_id", claims.UserID)
			ctx = context.WithValue(ctx, "email", claims.Email)

			// 透传 JWT 元信息（供后端服务做 token blacklist/version/idempotency 等控制）
			if claims.ID != "" {
				ctx = context.WithValue(ctx, JWTIDKey, claims.ID)
			}
			if claims.ExpiresAt != nil {
				ctx = context.WithValue(ctx, JWTExpiresAtKey, claims.ExpiresAt.Time.Unix())
			}
			if claims.Version > 0 {
				ctx = context.WithValue(ctx, JWTTokenVersionKey, claims.Version)
			}

			// 继续处理请求
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// OptionalJWTAuth 可选的 JWT 认证（token 存在则验证，不存在则跳过）
func OptionalJWTAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				// 没有 token，直接放行
				next.ServeHTTP(w, r)
				return
			}

			// 有 token，执行验证
			JWTAuth(secret)(next).ServeHTTP(w, r)
		})
	}
}

// GetUserID 从 context 中获取用户ID
func GetUserID(ctx context.Context) string {
	if userID, ok := ctx.Value("user_id").(string); ok {
		return userID
	}
	return ""
}

// GetEmail 从 context 中获取用户邮箱
func GetEmail(ctx context.Context) string {
	if email, ok := ctx.Value("email").(string); ok {
		return email
	}
	return ""
}

// GenerateToken 生成 JWT token（工具函数）
func GenerateToken(userID, email, secret string, expireDuration time.Duration) (string, error) {
	claims := JWTClaims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expireDuration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}
