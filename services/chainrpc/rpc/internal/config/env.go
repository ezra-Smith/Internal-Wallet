package config

import (
	"os"
	"strings"

	"github.com/zeromicro/go-zero/core/logx"
)

// ApplyEnvOverrides applies environment variable overrides to config.
//
// Supported environment variables:
// - REDIS_PASSWORD: Redis password (for CacheRedis[0])
// - ETH_RPC_API_KEY: API key for ETH RPC node authentication
// - BSC_RPC_API_KEY: API key for BSC RPC node authentication
// - TRON_API_KEY: API key for TRON node authentication
func ApplyEnvOverrides(c *Config) error {
	if c == nil {
		return nil
	}

	logx.Info("🔐 Applying environment variable overrides for chainrpc service...")
	loadedCount := 0

	// ETH RPC API Key
	if ethKey := strings.TrimSpace(os.Getenv("CHAINRPC_ETH_RPC_API_KEY")); ethKey != "" {
		for i := range c.Chains {
			if c.Chains[i].ChainType == "ETH" {
				c.Chains[i].APIKey = ethKey
				loadedCount++
				logx.Infof("✓ Loaded ETH_RPC_API_KEY from environment (length: %d)", len(ethKey))
				break
			}
		}
	}

	// BSC RPC API Key
	if bscKey := strings.TrimSpace(os.Getenv("CHAINRPC_BSC_RPC_API_KEY")); bscKey != "" {
		for i := range c.Chains {
			if c.Chains[i].ChainType == "BSC" {
				c.Chains[i].APIKey = bscKey
				loadedCount++
				logx.Infof("✓ Loaded BSC_RPC_API_KEY from environment (length: %d)", len(bscKey))
				break
			}
		}
	}

	// TRON API Key
	if tronKey := strings.TrimSpace(os.Getenv("CHAINRPC_TRON_API_KEY")); tronKey != "" {
		for i := range c.Chains {
			if c.Chains[i].ChainType == "TRON" {
				c.Chains[i].TronAPIKey = tronKey
				loadedCount++
				logx.Infof("✓ Loaded TRON_API_KEY from environment (length: %d)", len(tronKey))
				break
			}
		}
	}

	if loadedCount > 0 {
		logx.Infof("✓ Applied %d environment variable overrides", loadedCount)
	} else {
		logx.Info("⚠️  No environment variable overrides applied")
	}

	return nil
}
