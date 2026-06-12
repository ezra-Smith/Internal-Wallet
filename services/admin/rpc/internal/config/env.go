package config

import (
	"os"
	"strings"

	"github.com/zeromicro/go-zero/core/logx"
	"internalwallet/pkg/secrets"
)

// ApplySecretsConfig injects secrets from SecretConfig (preferred over env vars in AWS environments).
func ApplySecretsConfig(c *Config, sc *secrets.SecretConfig) {
	if c == nil || sc == nil {
		return
	}

	logx.Info("🔐 Applying secrets from SecretConfig for admin service...")
	loadedCount := 0

	// Admin Swap Admin Token
	if sc.Swap.AdminSwapAdminToken != "" {
		c.SwapAuth.AdminToken = sc.Swap.AdminSwapAdminToken
		loadedCount++
		logx.Infof("✓ Loaded admin_swap_admin_token from SecretConfig (length: %d)", len(sc.Swap.AdminSwapAdminToken))
	}

	logx.Infof("✓ Applied %d secrets from SecretConfig", loadedCount)
}

// ApplyEnvOverrides applies secret overrides from env vars.
//
// Env vars:
// - MYSQL_PASSWORD: MariaDB/MySQL password (for common/db)
// - REDIS_PASSWORD: Redis password (for CacheRedis[0])
// - ADMIN_JWT_ACCESS_SECRET: Admin JWT access secret
// - ADMIN_JWT_REFRESH_SECRET: Admin JWT refresh secret
// - ADMIN_SWAP_ADMIN_TOKEN: Swap Admin Token 用于 Admin 调用 Swap 管理接口时的认证
// - EMAIL_SMTP_HOST: SMTP 服务器地址
// - EMAIL_SMTP_PORT: SMTP 端口
// - EMAIL_FROM_ADDRESS: 发件人邮箱地址
// - EMAIL_FROM_PASSWORD: 发件人邮箱密码
// - EMAIL_FREEZE_ACCOUNT_URL: 冻结账号页面 URL
// - EMAIL_SUPPORT_EMAIL: 客服邮箱
// - EMAIL_COMPANY_NAME: 公司名称
func ApplyEnvOverrides(c *Config) {
	if c == nil {
		return
	}

	logx.Info("🔐 Applying environment variable overrides for admin service...")
	loadedCount := 0

	// MySQL password (recommended via Kubernetes Secret env)
	if v := strings.TrimSpace(os.Getenv("MYSQL_PASSWORD")); v != "" {
		c.MySQL.Password = v
		loadedCount++
		logx.Info("✓ Loaded MYSQL_PASSWORD from environment")
	} else {
		logx.Info("⚠️  MYSQL_PASSWORD not set in environment, using config file value")
	}

	// Redis password (CacheRedis[0].Pass)
	if v := strings.TrimSpace(os.Getenv("REDIS_PASSWORD")); v != "" {
		if len(c.CacheRedis) > 0 {
			c.CacheRedis[0].Pass = v
			loadedCount++
			logx.Info("✓ Loaded REDIS_PASSWORD from environment")
		} else {
			logx.Info("⚠️  REDIS_PASSWORD set but CacheRedis is not configured")
		}
	} else {
		logx.Info("⚠️  REDIS_PASSWORD not set in environment, using config file value")
	}

	// Admin JWT secrets (recommended via Kubernetes Secret env)
	if v := strings.TrimSpace(os.Getenv("ADMIN_JWT_ACCESS_SECRET")); v != "" {
		c.JWT.AccessSecret = v
		loadedCount++
		logx.Info("✓ Loaded ADMIN_JWT_ACCESS_SECRET from environment")
	} else {
		logx.Info("⚠️  ADMIN_JWT_ACCESS_SECRET not set in environment, using config file value")
	}
	if v := strings.TrimSpace(os.Getenv("ADMIN_JWT_REFRESH_SECRET")); v != "" {
		c.JWT.RefreshSecret = v
		loadedCount++
		logx.Info("✓ Loaded ADMIN_JWT_REFRESH_SECRET from environment")
	} else {
		logx.Info("⚠️  ADMIN_JWT_REFRESH_SECRET not set in environment, using config file value")
	}

	// 检查并加载 Swap Admin Token（用于 Admin 调用 Swap 服务）
	if v := strings.TrimSpace(os.Getenv("ADMIN_SWAP_ADMIN_TOKEN")); v != "" {
		c.SwapAuth.AdminToken = v
		loadedCount++
		logx.Infof("✓ Loaded ADMIN_SWAP_ADMIN_TOKEN from environment (length: %d)", len(v))
	} else {
		logx.Info("⚠️  ADMIN_SWAP_ADMIN_TOKEN not set in environment, using config file value")
	}

	// 邮件配置 - 从环境变量加载
	if v := strings.TrimSpace(os.Getenv("EMAIL_SMTP_HOST")); v != "" {
		c.Email.SMTPHost = v
		loadedCount++
		logx.Infof("✓ Loaded EMAIL_SMTP_HOST from environment: %s", v)
	}
	if v := strings.TrimSpace(os.Getenv("EMAIL_SMTP_PORT")); v != "" {
		c.Email.SMTPPort = v
		loadedCount++
		logx.Infof("✓ Loaded EMAIL_SMTP_PORT from environment: %s", v)
	}
	if v := strings.TrimSpace(os.Getenv("EMAIL_FROM_ADDRESS")); v != "" {
		c.Email.FromAddress = v
		loadedCount++
		logx.Infof("✓ Loaded EMAIL_FROM_ADDRESS from environment: %s", v)
	}
	if v := os.Getenv("EMAIL_FROM_PASSWORD"); strings.TrimSpace(v) != "" {
		// Gmail 应用专用密码可能包含空格，需要去掉
		c.Email.FromPassword = strings.ReplaceAll(strings.TrimSpace(v), " ", "")
		loadedCount++
		logx.Infof("✓ Loaded EMAIL_FROM_PASSWORD from environment (length: %d)", len(c.Email.FromPassword))
	}
	if v := strings.TrimSpace(os.Getenv("EMAIL_FREEZE_ACCOUNT_URL")); v != "" {
		c.Email.FreezeAccountURL = v
		loadedCount++
		logx.Infof("✓ Loaded EMAIL_FREEZE_ACCOUNT_URL from environment")
	}
	if v := strings.TrimSpace(os.Getenv("EMAIL_SUPPORT_EMAIL")); v != "" {
		c.Email.SupportEmail = v
		loadedCount++
		logx.Infof("✓ Loaded EMAIL_SUPPORT_EMAIL from environment: %s", v)
	}
	if v := strings.TrimSpace(os.Getenv("EMAIL_COMPANY_NAME")); v != "" {
		c.Email.CompanyName = v
		loadedCount++
		logx.Infof("✓ Loaded EMAIL_COMPANY_NAME from environment: %s", v)
	}

	logx.Infof("✓ Applied %d environment variable overrides", loadedCount)

	// 输出配置状态（隐藏敏感信息）
	logx.Infof("MySQL.Password configured: %v", strings.TrimSpace(c.MySQL.Password) != "")
	if len(c.CacheRedis) > 0 {
		logx.Infof("CacheRedis[0].Pass configured: %v", strings.TrimSpace(c.CacheRedis[0].Pass) != "")
	}
	logx.Infof("JWT.AccessSecret configured: %v", strings.TrimSpace(c.JWT.AccessSecret) != "")
	logx.Infof("JWT.RefreshSecret configured: %v", strings.TrimSpace(c.JWT.RefreshSecret) != "")
	logx.Infof("SwapAuth.AdminToken configured: %v", c.SwapAuth.AdminToken != "")
	logx.Infof("Email.SMTPHost configured: %v", c.Email.SMTPHost != "")
	logx.Infof("Email.FromAddress configured: %v", c.Email.FromAddress != "")
}
