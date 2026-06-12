package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/zeromicro/go-zero/core/logx"
	"internalwallet/pkg/secrets"
)

// ApplySecretsConfig injects secrets from SecretConfig (preferred over env vars in AWS environments).
func ApplySecretsConfig(c *Config, sc *secrets.SecretConfig) {
	if c == nil || sc == nil {
		return
	}

	logx.Info("🔐 Applying secrets from SecretConfig for business service...")
	loadedCount := 0

	// Business Swap API Key
	if sc.Swap.BusinessSwapApiKey != "" {
		c.SwapAuth.ApiKey = sc.Swap.BusinessSwapApiKey
		loadedCount++
		logx.Infof("✓ Loaded business_swap_api_key from SecretConfig (length: %d)", len(sc.Swap.BusinessSwapApiKey))
	}

	// Geetest (optional)
	if sc.Geetest.CaptchaID != "" {
		c.Geetest.CaptchaID = sc.Geetest.CaptchaID
		loadedCount++
	}
	if sc.Geetest.CaptchaKey != "" {
		c.Geetest.CaptchaKey = sc.Geetest.CaptchaKey
		loadedCount++
	}
	if sc.Geetest.APIServer != "" {
		c.Geetest.APIServer = sc.Geetest.APIServer
		loadedCount++
	}
	if sc.Geetest.Timeout > 0 {
		c.Geetest.Timeout = sc.Geetest.Timeout
		loadedCount++
	}
	if sc.Geetest.CaptchaID != "" || sc.Geetest.CaptchaKey != "" || sc.Geetest.APIServer != "" || sc.Geetest.Timeout > 0 {
		c.Geetest.Enabled = sc.Geetest.Enabled
		c.Geetest.FailOpen = sc.Geetest.FailOpen
		loadedCount += 2
	}

	logx.Infof("✓ Applied %d secrets from SecretConfig", loadedCount)
}

// ApplyEnvOverrides applies secret overrides from env vars.
//
// Env vars:
// - BUSINESS_SWAP_API_KEY: Swap API Key 用于 Business 调用 Swap 服务时的认证
func ApplyEnvOverrides(c *Config) {
	if c == nil {
		return
	}

	logx.Info("🔐 Applying environment variable overrides for business service...")
	loadedCount := 0

	// 检查并加载 Swap API Key（用于 Business 调用 Swap 服务）
	if v := strings.TrimSpace(os.Getenv("BUSINESS_SWAP_API_KEY")); v != "" {
		c.SwapAuth.ApiKey = v
		loadedCount++
		logx.Infof("✓ Loaded BUSINESS_SWAP_API_KEY from environment (length: %d)", len(v))
	} else {
		logx.Info("⚠️  BUSINESS_SWAP_API_KEY not set in environment, using config file value")
	}

	// Geetest
	if v := strings.TrimSpace(os.Getenv("GEETEST_CAPTCHA_ID")); v != "" {
		c.Geetest.CaptchaID = v
		loadedCount++
		logx.Infof("✓ Loaded GEETEST_CAPTCHA_ID from environment (length: %d)", len(v))
	}
	if v := strings.TrimSpace(os.Getenv("GEETEST_CAPTCHA_KEY")); v != "" {
		c.Geetest.CaptchaKey = v
		loadedCount++
		logx.Infof("✓ Loaded GEETEST_CAPTCHA_KEY from environment (length: %d)", len(v))
	}
	if v := strings.TrimSpace(os.Getenv("GEETEST_API_SERVER")); v != "" {
		c.Geetest.APIServer = v
		loadedCount++
		logx.Infof("✓ Loaded GEETEST_API_SERVER from environment")
	}
	if v := strings.TrimSpace(os.Getenv("GEETEST_TIMEOUT_MS")); v != "" {
		if ms, err := parseInt64(v); err == nil && ms > 0 {
			c.Geetest.Timeout = ms
			loadedCount++
			logx.Infof("✓ Loaded GEETEST_TIMEOUT_MS from environment")
		}
	}
	if v := strings.TrimSpace(os.Getenv("GEETEST_ENABLED")); v != "" {
		if b, ok := parseBool(v); ok {
			c.Geetest.Enabled = b
			loadedCount++
			logx.Infof("✓ Loaded GEETEST_ENABLED from environment")
		}
	}
	if v := strings.TrimSpace(os.Getenv("GEETEST_FAIL_OPEN")); v != "" {
		if b, ok := parseBool(v); ok {
			c.Geetest.FailOpen = b
			loadedCount++
			logx.Infof("✓ Loaded GEETEST_FAIL_OPEN from environment")
		}
	}

	logx.Infof("✓ Applied %d environment variable overrides", loadedCount)

	// 输出配置状态（隐藏敏感信息）
	logx.Infof("SwapAuth.ApiKey configured: %v", c.SwapAuth.ApiKey != "")
	logx.Infof("Geetest configured: %v (enabled=%v)", c.Geetest.CaptchaID != "" && c.Geetest.CaptchaKey != "", c.Geetest.Enabled)
}

func parseBool(value string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "y", "on":
		return true, true
	case "0", "false", "no", "n", "off":
		return false, true
	default:
		return false, false
	}
}

func parseInt64(value string) (int64, error) {
	return strconv.ParseInt(strings.TrimSpace(value), 10, 64)
}
