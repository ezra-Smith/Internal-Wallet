package secrets

import (
	"context"
	"fmt"
	"os"
	"sync"
)

var (
	globalConfig *SecretConfig
	configMutex  sync.RWMutex
)

// Load 加载配置（主入口）
// 自动检测环境：优先使用 APP_ENV 环境变量，否则使用 "local"
func Load(ctx context.Context) (*SecretConfig, error) {
	env := getEnvironment()
	return LoadWithEnv(ctx, env)
}

// LoadWithMode 使用 Go-Zero Mode 字段加载配置（推荐）
// mode: pro
func LoadWithMode(ctx context.Context, mode string) (*SecretConfig, error) {
	env := mapModeToEnvironment(mode)
	return LoadWithEnv(ctx, env)
}

// LoadWithEnv 使用指定环境加载配置
// env: local/production
func LoadWithEnv(ctx context.Context, env string) (*SecretConfig, error) {
	var config *SecretConfig
	var err error

	switch env {
	case "local":
		// 本地开发：从 .env 文件加载
		config, err = loadFromLocal()
		if err != nil {
			return nil, fmt.Errorf("failed to load from local: %w", err)
		}
		fmt.Printf("✅ [SECRETS] Loaded from local .env (environment: %s)\n", env)

	case "production":
		// 生产环境：从 CSI secrets 目录加载（Secrets Manager 由 CSI provider 获取）
		config, err = loadFromFiles(defaultSecretsMountDir)
		if err != nil {
			return nil, fmt.Errorf("failed to load from secrets mount: %w", err)
		}
		fmt.Printf("✅ [SECRETS] Loaded from CSI secrets mount (environment: %s)\n", env)

	default:
		return nil, fmt.Errorf("unknown environment: %s (valid: local, production)", env)
	}

	// 验证配置
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	// 缓存到全局变量
	configMutex.Lock()
	globalConfig = config
	configMutex.Unlock()

	return config, nil
}

// GetConfig 获取当前配置（带缓存）
func GetConfig() *SecretConfig {
	configMutex.RLock()
	defer configMutex.RUnlock()
	return globalConfig
}

// MustLoad 加载配置，失败则 panic
func MustLoad(ctx context.Context) *SecretConfig {
	config, err := Load(ctx)
	if err != nil {
		panic(fmt.Sprintf("FATAL: Failed to load secrets: %v", err))
	}
	return config
}

// MustLoadWithMode 使用 Mode 字段加载配置，失败则 panic
func MustLoadWithMode(ctx context.Context, mode string) *SecretConfig {
	config, err := LoadWithMode(ctx, mode)
	if err != nil {
		panic(fmt.Sprintf("FATAL: Failed to load secrets: %v", err))
	}
	return config
}

// getEnvironment 获取当前环境（优先级链）
func getEnvironment() string {
	// 优先级 1: APP_ENV 环境变量
	if env := os.Getenv("APP_ENV"); env != "" {
		return env
	}

	// 优先级 2: 默认 production（生产环境唯一入口）
	return "production"
}

// mapModeToEnvironment 将 Go-Zero Mode 映射到环境名称
func mapModeToEnvironment(mode string) string {
	switch mode {
	case "pro", "production":
		return "production" // 生产环境使用 AWS production 配置
	case "dev", "test":
		return "local" // 开发/测试环境使用本地配置
	default:
		return "production" // 未知模式默认 production
	}
}

// Validate 验证配置的完整性和安全性
func (c *SecretConfig) Validate() error {
	env := os.Getenv("APP_ENV")

	// 生产/预发环境需要强密码验证
	if env == "production" {
		return c.validateProduction()
	}

	// 本地开发环境宽松验证
	return c.validateLocal()
}

// validateProduction 生产环境严格验证
func (c *SecretConfig) validateProduction() error {
	// 1. 验证 Signer 加密密码（只有 signer 服务需要）
	if c.Security.Signer.EncryptionPassword != "" && len(c.Security.Signer.EncryptionPassword) < 32 {
		return fmt.Errorf("CRITICAL: signer encryption password must be at least 32 characters (got %d)", len(c.Security.Signer.EncryptionPassword))
	}

	// 2. 验证数据库密码
	if len(c.Database.MySQL.Password) < 16 {
		return fmt.Errorf("MySQL password must be at least 16 characters (got %d)", len(c.Database.MySQL.Password))
	}

	// 3. 验证 Redis 密码
	if len(c.Cache.Redis.Password) < 16 {
		return fmt.Errorf("Redis password must be at least 16 characters (got %d)", len(c.Cache.Redis.Password))
	}

	// 4. 验证 JWT 密钥
	if len(c.Security.JWT.AccessSecret) < 32 {
		return fmt.Errorf("JWT access secret must be at least 32 characters (got %d)", len(c.Security.JWT.AccessSecret))
	}
	if len(c.Security.JWT.RefreshSecret) < 32 {
		return fmt.Errorf("JWT refresh secret must be at least 32 characters (got %d)", len(c.Security.JWT.RefreshSecret))
	}

	// 5. 验证 Admin JWT 密钥
	if len(c.Security.AdminJWT.AccessSecret) < 32 {
		return fmt.Errorf("Admin JWT access secret must be at least 32 characters (got %d)", len(c.Security.AdminJWT.AccessSecret))
	}
	if len(c.Security.AdminJWT.RefreshSecret) < 32 {
		return fmt.Errorf("Admin JWT refresh secret must be at least 32 characters (got %d)", len(c.Security.AdminJWT.RefreshSecret))
	}

	return nil
}

// validateLocal 本地开发宽松验证
func (c *SecretConfig) validateLocal() error {
	// 本地环境宽松验证：只检查真正必需的配置
	// 注意：某些服务（如 chainrpc、market）不需要 MySQL/Signer/JWT，所以这些配置为空时只给出警告

	// MySQL 密码（某些服务不需要）
	if c.Database.MySQL.Password == "" {
		fmt.Println("⚠️  WARNING: MYSQL_PASSWORD is not set")
		fmt.Println("⚠️  This is OK for services that don't need MySQL (e.g., chainrpc, market)")
	}

	// Signer 加密密码（只有 signer 服务需要）
	if c.Security.Signer.EncryptionPassword == "" {
		fmt.Println("⚠️  WARNING: SIGNER_ENCRYPTION_PASSWORD is not set")
		fmt.Println("⚠️  This is OK for services that don't need signer encryption (e.g., chainrpc, market)")
	} else if len(c.Security.Signer.EncryptionPassword) < 32 {
		// 给出弱密码警告（但不阻止启动）
		fmt.Println("⚠️  WARNING: Signer encryption password is weak (< 32 chars)")
		fmt.Println("⚠️  This is OK for local development, but NEVER use in production!")
	}

	// JWT 密钥（只有需要 JWT 的服务需要，如 business、admin）
	if c.Security.JWT.AccessSecret == "" {
		fmt.Println("⚠️  WARNING: JWT_ACCESS_SECRET is not set")
		fmt.Println("⚠️  This is OK for services that don't need JWT (e.g., chainrpc, market)")
	}

	// Redis 密码（大多数服务都需要，但允许为空用于无密码的本地 Redis）
	if c.Cache.Redis.Password == "" {
		fmt.Println("ℹ️  INFO: REDIS_PASSWORD is not set (using no password)")
	}

	return nil
}
