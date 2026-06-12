package utils

import (
	"os"
	"strconv"
	"strings"
)

// LoadNodeIDFromEnv 从环境变量读取 NodeID（用于 Snowflake 或服务标识）。
//
// - key: 环境变量名
// - fallback: 当环境变量未设置或解析失败时返回的默认值
func LoadNodeIDFromEnv(key string, fallback int64) int64 {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}

	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return fallback
	}

	return v
}

