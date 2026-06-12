package config

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/zrpc"
	commonDB "internalwallet/common/db" // 使用通用的 MySQL 配置
)

type Config struct {
	zrpc.RpcServerConf

	// 雪花ID配置（必须配置，范围0-1023，每个服务唯一）
	NodeID int64 `json:",optional"`

	// GORM MySQL 配置（使用通用配置）
	MySQL commonDB.MySQLConfig `json:",optional"`

	// Redis 缓存配置
	CacheRedis cache.CacheConf

	// 安全配置
	Security SecurityConfig

	// 数据库自动迁移配置
	AutoMigrate bool `json:",optional,default=false"` // 是否启用自动建表（仅开发环境建议启用）
}

type SecurityConfig struct {
	EncryptionPassword string   `json:",optional"` // Seed加密密码
	KeyStorePath       string   `json:",optional"` // 私钥存储路径
	EnableTEE          bool     `json:",optional"` // 是否启用TEE
	EnableHSM          bool     `json:",optional"` // 是否启用硬件安全模块
	EnableIPWhitelist  bool     `json:",optional"` // 是否启用IP白名单（默认false）
	AllowedIPs         []string `json:",optional"` // IP白名单（支持单个IP：127.0.0.1）
	AllowedCIDRs       []string `json:",optional"` // CIDR白名单（支持网段：10.0.1.0/24）
}
