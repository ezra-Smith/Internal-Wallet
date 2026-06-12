package secrets

import (
	"context"
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

// ModeConfig 最小配置结构（仅用于读取 Mode）
type ModeConfig struct {
	Mode string `yaml:"Mode"`
}

// ReadModeFromConfig 从配置文件快速读取 Mode 字段
// 用于在加载密钥前获取环境信息
func ReadModeFromConfig(configPath string) (string, error) {
	// 1. 读取文件
	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", fmt.Errorf("failed to read config file: %w", err)
	}

	// 2. 解析 Mode 字段
	var cfg ModeConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return "", fmt.Errorf("failed to parse config file: %w", err)
	}

	// 3. 返回 Mode（如果为空，默认 pro）
	if cfg.Mode == "" {
		return "pro", nil
	}

	return cfg.Mode, nil
}

// LoadFromConfigMode 从配置文件的 Mode 字段加载密钥（便捷方法）
// 使用示例：
//
//	secretConfig, err := secrets.LoadFromConfigMode(ctx, "etc/signer.yaml")
func LoadFromConfigMode(ctx context.Context, configPath string) (*SecretConfig, error) {
	// APP_ENV has the highest priority (k8s / prod deployments should set it explicitly).
	if env := os.Getenv("APP_ENV"); env != "" {
		fmt.Printf("📄 Using APP_ENV from environment: %s\n", env)
		return LoadWithEnv(ctx, env)
	}

	// 1. 读取 Mode
	mode, err := ReadModeFromConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read mode from config: %w", err)
	}

	fmt.Printf("📄 Read Mode from config: %s\n", mode)

	// 2. 根据 Mode 加载密钥
	return LoadWithMode(ctx, mode)
}
