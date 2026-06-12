package config

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"

	"internalwallet/pkg/secrets"
	"github.com/zeromicro/go-zero/core/logx"
)

// ApplySecretsConfig injects secrets from SecretConfig (preferred over env vars in AWS environments).
func ApplySecretsConfig(c *Config, sc *secrets.SecretConfig) {
	if c == nil || sc == nil {
		return
	}

	logx.Info("🔐 Applying secrets from SecretConfig for swap service...")
	loadedCount := 0

	// Swap API Key Pepper
	if sc.Swap.ApiKeyPepper != "" {
		c.Swap.Auth.ApiKeyPepper = sc.Swap.ApiKeyPepper
		loadedCount++
		logx.Info("✓ Loaded swap_api_key_pepper from SecretConfig")
	}

	// Swap Admin Token SHA256
	if sc.Swap.AdminTokenSHA256 != "" {
		c.Swap.Auth.AdminTokenHash = strings.ToLower(sc.Swap.AdminTokenSHA256)
		loadedCount++
		logx.Info("✓ Loaded swap_admin_token_sha256 from SecretConfig")
	}

	// Swap Provider Type
	if sc.Swap.ProviderType != "" {
		c.Swap.Provider.Type = sc.Swap.ProviderType
		loadedCount++
		logx.Infof("✓ Loaded swap_provider_type from SecretConfig: %s", sc.Swap.ProviderType)
	}

	// OKX credentials
	if sc.Swap.OKXApiKey != "" {
		c.Swap.Provider.Okx.ApiKey = sc.Swap.OKXApiKey
		loadedCount++
		logx.Infof("✓ Loaded okx_api_key from SecretConfig (length: %d)", len(sc.Swap.OKXApiKey))
	}
	if sc.Swap.OKXSecretKey != "" {
		c.Swap.Provider.Okx.SecretKey = sc.Swap.OKXSecretKey
		loadedCount++
		logx.Infof("✓ Loaded okx_secret_key from SecretConfig (length: %d)", len(sc.Swap.OKXSecretKey))
	}
	if sc.Swap.OKXPassphrase != "" {
		c.Swap.Provider.Okx.Passphrase = sc.Swap.OKXPassphrase
		loadedCount++
		logx.Infof("✓ Loaded okx_passphrase from SecretConfig (length: %d)", len(sc.Swap.OKXPassphrase))
	}

	// 1inch API Key
	if sc.Swap.OneInchApiKey != "" {
		c.Swap.Provider.OneInch.ApiKey = sc.Swap.OneInchApiKey
		loadedCount++
		logx.Info("✓ Loaded oneinch_api_key from SecretConfig")
	}

	logx.Infof("✓ Applied %d secrets from SecretConfig", loadedCount)
}

// ApplyEnvOverrides applies secret overrides from env vars.
//
// Env vars:
// - MYSQL_PASSWORD: MariaDB/MySQL password (for common/db)
// - REDIS_PASSWORD: Redis password (for CacheRedis[0])
// - ONEINCH_API_KEY: 1inch upstream API key
// - OKX_API_KEY: OKX API key (OK-ACCESS-KEY)
// - OKX_SECRET_KEY: OKX secret key (used to generate OK-ACCESS-SIGN)
// - OKX_PASSPHRASE: OKX passphrase (OK-ACCESS-PASSPHRASE)
// - SWAP_PROVIDER_TYPE: swap provider type (e.g. "1inch" / "okx")
// - SWAP_API_KEY_PEPPER: pepper for hashing client API keys
// - SWAP_ADMIN_TOKEN: admin token (plaintext; will be hashed into AdminTokenHash)
// - SWAP_ADMIN_TOKEN_SHA256: admin token hash (sha256 hex); takes precedence over SWAP_ADMIN_TOKEN
func ApplyEnvOverrides(c *Config) {
	if c == nil {
		return
	}

	logx.Info("🔐 Applying environment variable overrides for swap service...")
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

	// 检查并加载 1inch API Key
	if v := strings.TrimSpace(os.Getenv("ONEINCH_API_KEY")); v != "" {
		c.Swap.Provider.OneInch.ApiKey = v
		loadedCount++
		logx.Info("✓ Loaded ONEINCH_API_KEY from environment")
	} else {
		logx.Info("⚠️  ONEINCH_API_KEY not set in environment")
	}

	// 检查并加载 Provider Type
	if v := strings.TrimSpace(os.Getenv("SWAP_PROVIDER_TYPE")); v != "" {
		c.Swap.Provider.Type = v
		loadedCount++
		logx.Infof("✓ Loaded SWAP_PROVIDER_TYPE from environment: %s", v)
	} else {
		logx.Infof("⚠️  SWAP_PROVIDER_TYPE not set, using config default: %s", c.Swap.Provider.Type)
	}

	// 检查并加载 OKX API Key
	if v := strings.TrimSpace(os.Getenv("OKX_API_KEY")); v != "" {
		c.Swap.Provider.Okx.ApiKey = v
		loadedCount++
		logx.Infof("✓ Loaded OKX_API_KEY from environment (length: %d)", len(v))
	} else {
		logx.Info("⚠️  OKX_API_KEY not set in environment")
	}

	// 检查并加载 OKX Secret Key
	if v := strings.TrimSpace(os.Getenv("OKX_SECRET_KEY")); v != "" {
		c.Swap.Provider.Okx.SecretKey = v
		loadedCount++
		logx.Infof("✓ Loaded OKX_SECRET_KEY from environment (length: %d)", len(v))
	} else {
		logx.Info("⚠️  OKX_SECRET_KEY not set in environment")
	}

	// 检查并加载 OKX Passphrase
	if v := strings.TrimSpace(os.Getenv("OKX_PASSPHRASE")); v != "" {
		c.Swap.Provider.Okx.Passphrase = v
		loadedCount++
		logx.Infof("✓ Loaded OKX_PASSPHRASE from environment (length: %d)", len(v))
	} else {
		logx.Info("⚠️  OKX_PASSPHRASE not set in environment")
	}

	// 检查并加载 API Key Pepper
	if v := strings.TrimSpace(os.Getenv("SWAP_API_KEY_PEPPER")); v != "" {
		c.Swap.Auth.ApiKeyPepper = v
		loadedCount++
		logx.Info("✓ Loaded SWAP_API_KEY_PEPPER from environment")
	} else {
		logx.Info("⚠️  SWAP_API_KEY_PEPPER not set in environment")
	}

	// 检查并加载 Admin Token
	if v := strings.TrimSpace(os.Getenv("SWAP_ADMIN_TOKEN_SHA256")); v != "" {
		c.Swap.Auth.AdminTokenHash = strings.ToLower(v)
		loadedCount++
		logx.Info("✓ Loaded SWAP_ADMIN_TOKEN_SHA256 from environment")
	} else if v := os.Getenv("SWAP_ADMIN_TOKEN"); strings.TrimSpace(v) != "" && strings.TrimSpace(c.Swap.Auth.AdminTokenHash) == "" {
		sum := sha256.Sum256([]byte(v))
		c.Swap.Auth.AdminTokenHash = hex.EncodeToString(sum[:])
		loadedCount++
		logx.Info("✓ Loaded and hashed SWAP_ADMIN_TOKEN from environment")
	} else {
		logx.Info("⚠️  SWAP_ADMIN_TOKEN/SWAP_ADMIN_TOKEN_SHA256 not set in environment")
	}

	logx.Infof("✓ Applied %d environment variable overrides", loadedCount)

	// 输出配置状态（隐藏敏感信息）
	logx.Infof("Provider Type: %s", c.Swap.Provider.Type)
	logx.Infof("OKX ApiKey configured: %v", c.Swap.Provider.Okx.ApiKey != "")
	logx.Infof("OKX SecretKey configured: %v", c.Swap.Provider.Okx.SecretKey != "")
	logx.Infof("OKX Passphrase configured: %v", c.Swap.Provider.Okx.Passphrase != "")
	logx.Infof("1inch ApiKey configured: %v", c.Swap.Provider.OneInch.ApiKey != "")
	logx.Infof("Auth ApiKeyPepper configured: %v", c.Swap.Auth.ApiKeyPepper != "")
	logx.Infof("Auth AdminTokenHash configured: %v", c.Swap.Auth.AdminTokenHash != "")
}
