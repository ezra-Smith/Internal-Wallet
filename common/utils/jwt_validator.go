package utils

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"

	"github.com/zeromicro/go-zero/core/logx"
)

const (
	// MinJWTSecretLength JWT密钥最小长度（字节）
	MinJWTSecretLength = 32
	// RecommendedJWTSecretLength JWT密钥推荐长度（字节）
	RecommendedJWTSecretLength = 64
)

// ValidateJWTSecret 验证JWT密钥强度
// 如果密钥不符合安全要求，返回错误
func ValidateJWTSecret(secret string, secretName string) error {
	if secret == "" {
		return fmt.Errorf("%s is empty - JWT authentication will not work", secretName)
	}

	// 先检查是否使用了默认/示例密钥（优先级最高）
	dangerousSecrets := []string{
		"your-secret-key-change-me-in-production",
		"your-refresh-secret-change-me-in-production",
		"internalwallet-secret",
		"secret",
		"password",
		"123456",
		"change-me",
		"default-secret",
	}

	for _, dangerous := range dangerousSecrets {
		if secret == dangerous {
			return fmt.Errorf("%s is using a default/example value - this is a critical security risk", secretName)
		}
	}

	secretLen := len(secret)

	// 检查最小长度
	if secretLen < MinJWTSecretLength {
		return fmt.Errorf("%s is too short (%d bytes). Minimum required: %d bytes",
			secretName, secretLen, MinJWTSecretLength)
	}

	// 警告：密钥长度低于推荐值
	if secretLen < RecommendedJWTSecretLength {
		logx.Infof("⚠ WARNING: %s length is %d bytes. Recommended: %d+ bytes for maximum security",
			secretName, secretLen, RecommendedJWTSecretLength)
	}

	return nil
}

// MustValidateJWTSecret 验证JWT密钥，如果失败则退出程序
// 用于服务启动时的关键验证
func MustValidateJWTSecret(secret string, secretName string) {
	if err := ValidateJWTSecret(secret, secretName); err != nil {
		logx.Severef("❌ CRITICAL: %v", err)
		logx.Severef("Service cannot start with weak or missing JWT secret")
		logx.Severef("Please set a strong %s (minimum %d bytes, recommended %d+ bytes)",
			secretName, MinJWTSecretLength, RecommendedJWTSecretLength)
		logx.Severef("Generate a strong secret with: openssl rand -base64 64")
		os.Exit(1)
	}

	logx.Infof("✓ %s validated successfully (%d bytes)", secretName, len(secret))
}

// GenerateJWTSecret 生成一个安全的JWT密钥
// length: 生成密钥的字节长度（推荐64字节）
func GenerateJWTSecret(length int) (string, error) {
	if length < MinJWTSecretLength {
		return "", fmt.Errorf("requested length %d is less than minimum %d", length, MinJWTSecretLength)
	}

	randomBytes := make([]byte, length)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// 使用base64编码，使其可以在配置文件中使用
	return base64.StdEncoding.EncodeToString(randomBytes), nil
}

// ValidateJWTSecretPair 验证访问令牌和刷新令牌密钥
// 确保两个密钥不同，且都符合安全要求
func ValidateJWTSecretPair(accessSecret, refreshSecret string) error {
	// 验证访问令牌密钥
	if err := ValidateJWTSecret(accessSecret, "JWT Access Secret"); err != nil {
		return err
	}

	// 验证刷新令牌密钥
	if err := ValidateJWTSecret(refreshSecret, "JWT Refresh Secret"); err != nil {
		return err
	}

	// 确保两个密钥不同
	if accessSecret == refreshSecret {
		return fmt.Errorf("access secret and refresh secret must be different")
	}

	return nil
}

// MustValidateJWTSecretPair 验证JWT密钥对，如果失败则退出程序
func MustValidateJWTSecretPair(accessSecret, refreshSecret string) {
	if err := ValidateJWTSecretPair(accessSecret, refreshSecret); err != nil {
		logx.Severef("❌ CRITICAL: %v", err)
		logx.Severef("Service cannot start with weak or invalid JWT secrets")
		logx.Severef("Generate strong secrets with:")
		logx.Severef("  Access Secret:  openssl rand -base64 64")
		logx.Severef("  Refresh Secret: openssl rand -base64 64")
		os.Exit(1)
	}

	logx.Infof("✓ JWT secret pair validated successfully")
}
