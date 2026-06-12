package security

import (
	"fmt"
	"strconv"
	"time"

	"internalwallet/common/middleware"
	"internalwallet/common/utils"

	"github.com/golang-jwt/jwt/v4"
)

type TokenMeta struct {
	JTI string
	Exp int64
}

// GenerateAdminAccessToken 生成 Admin Access Token（HS256）
func GenerateAdminAccessToken(adminID int64, username string, tokenVersion int64, secret string, expireSeconds int64) (string, TokenMeta, error) {
	return generateToken(adminID, username, tokenVersion, secret, expireSeconds, "internalwallet-admin", []string{"admin"})
}

// GenerateAdmin2FATempToken 生成 2FA 临时 Token（HS256，默认 5 分钟）
func GenerateAdmin2FATempToken(adminID int64, username string, tokenVersion int64, secret string, expireSeconds int64) (string, TokenMeta, error) {
	return generateToken(adminID, username, tokenVersion, secret, expireSeconds, "internalwallet-admin-2fa", []string{"admin-2fa"})
}

func generateToken(adminID int64, username string, tokenVersion int64, secret string, expireSeconds int64, issuer string, audience []string) (string, TokenMeta, error) {
	if secret == "" {
		return "", TokenMeta{}, fmt.Errorf("jwt secret is empty")
	}
	if expireSeconds <= 0 {
		expireSeconds = 3600
	}

	jti := utils.GenerateIDString()
	now := time.Now()
	expAt := now.Add(time.Duration(expireSeconds) * time.Second)

	claims := middleware.JWTClaims{
		UserID:  strconv.FormatInt(adminID, 10),
		Email:   username,
		Version: tokenVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			Issuer:    issuer,
			Audience:  audience,
			ExpiresAt: jwt.NewNumericDate(expAt),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", TokenMeta{}, err
	}
	return signed, TokenMeta{JTI: jti, Exp: expAt.Unix()}, nil
}

// ParseToken 解析并验证 token（仅用于 2FA 临时 token 等需要服务端自行验证的场景）
func ParseToken(tokenString string, secret string) (*middleware.JWTClaims, error) {
	parsed, err := jwt.ParseWithClaims(tokenString, &middleware.JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	if !parsed.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	claims, ok := parsed.Claims.(*middleware.JWTClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}
