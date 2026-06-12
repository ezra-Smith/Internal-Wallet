package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/zeromicro/go-zero/core/logx"
)

// LoadEnvVariables 从环境变量加载敏感配置，覆盖配置文件中的值
func LoadEnvVariables(cfg *Config) {
	logx.Info("🔐 Loading sensitive configuration from environment variables...")

	loadedCount := 0

	// 数据库密码
	if dbPass := os.Getenv("CHAINSYNC_MYSQL_PASSWORD"); dbPass != "" {
		cfg.MySQL.Password = dbPass
		loadedCount++
		logx.Debug("Loaded MySQL password from environment variable")
	}

	// Redis 密码
	if redisPass := os.Getenv("CHAINSYNC_REDIS_PASSWORD"); redisPass != "" {
		for i := range cfg.CacheRedis {
			cfg.CacheRedis[i].Pass = redisPass
		}
		loadedCount++
		logx.Debug("Loaded Redis password from environment variable")
	}

	// Kafka 密码
	if kafkaPass := os.Getenv("CHAINSYNC_KAFKA_PASSWORD"); kafkaPass != "" {
		cfg.Kafka.Password = kafkaPass
		loadedCount++
		logx.Debug("Loaded Kafka password from environment variable")
	}

	// Provider API Keys
	loadedCount += loadProviderAPIKey(&cfg.Providers.Ethereum, "CHAINSYNC_ETHEREUM_API_KEY")
	loadedCount += loadProviderAPIKey(&cfg.Providers.QuickNode, "CHAINSYNC_QUICKNODE_API_KEY")
	loadedCount += loadProviderAPIKey(&cfg.Providers.Infura, "CHAINSYNC_INFURA_API_KEY")
	loadedCount += loadProviderAPIKey(&cfg.Providers.Alchemy, "CHAINSYNC_ALCHEMY_API_KEY")
	loadedCount += loadProviderAPIKey(&cfg.Providers.BSC, "CHAINSYNC_BSC_API_KEY")
	loadedCount += loadProviderAPIKey(&cfg.Providers.Tron, "CHAINSYNC_TRON_API_KEY")

	// 自建节点 API Key (可以为所有自建节点设置统一的 API Key)
	if selfHostedAPIKey := os.Getenv("CHAINSYNC_SELFHOSTED_API_KEY"); selfHostedAPIKey != "" {
		for i := range cfg.Providers.SelfHosted {
			if cfg.Providers.SelfHosted[i].APIKey == "" {
				cfg.Providers.SelfHosted[i].APIKey = selfHostedAPIKey
			}
		}
		loadedCount++
		logx.Debug("Loaded SelfHosted API key from environment variable")
	}

	// 单独设置每个自建节点的 API Key
	// 格式: CHAINSYNC_SELFHOSTED_0_API_KEY, CHAINSYNC_SELFHOSTED_1_API_KEY, ...
	for i := range cfg.Providers.SelfHosted {
		envKey := os.Getenv(fmt.Sprintf("CHAINSYNC_SELFHOSTED_%d_API_KEY", i))
		if envKey != "" {
			cfg.Providers.SelfHosted[i].APIKey = envKey
			loadedCount++
			logx.Debugf("Loaded SelfHosted[%d] API key from environment variable", i)
		}
	}

	// 启用/禁用特定链 (例如: CHAINSYNC_ENABLED_CHAINS=ethereum,bsc,tron)
	if enabledChainsStr := os.Getenv("CHAINSYNC_ENABLED_CHAINS"); enabledChainsStr != "" {
		enabledChainsMap := make(map[string]bool)
		for _, chainName := range strings.Split(enabledChainsStr, ",") {
			enabledChainsMap[strings.ToLower(strings.TrimSpace(chainName))] = true
		}

		cfg.Chains.Ethereum.Enabled = enabledChainsMap["ethereum"] || enabledChainsMap["eth"]
		cfg.Chains.BSC.Enabled = enabledChainsMap["bsc"]
		cfg.Chains.Tron.Enabled = enabledChainsMap["tron"]
		loadedCount++
		logx.Infof("Enabled chains from environment variable: %s", enabledChainsStr)
	}

	logx.Infof("✅ Loaded %d configuration items from environment variables", loadedCount)
}

// loadProviderAPIKey 从环境变量加载单个 Provider 的 API Key
func loadProviderAPIKey(providerConfig *ProviderConfig, envVarName string) int {
	if apiKey := os.Getenv(envVarName); apiKey != "" {
		providerConfig.APIKey = apiKey
		logx.Debugf("Loaded API key for %s from environment variable %s", providerConfig.Name, envVarName)
		return 1
	}
	return 0
}
