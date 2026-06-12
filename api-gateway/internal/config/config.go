// Pure Gateway Configuration
// 纯网关配置 - 移除了所有数据层依赖

package config

import (
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

type S3Config struct {
	Enabled         bool   `json:",optional"`
	Region          string `json:",optional"`
	Bucket          string `json:",optional"`
	Endpoint        string `json:",optional"` // optional: S3-compatible endpoint (e.g. MinIO)
	AccessKeyID     string `json:",optional"`
	SecretAccessKey string `json:",optional"`
	SessionToken    string `json:",optional"`
	PublicBaseURL   string `json:",optional"` // optional: CDN/public host base, e.g. https://cdn.example.com
	Prefix          string `json:",optional"` // optional: key prefix folder, e.g. "admin"
	UsePathStyle    bool   `json:",optional"` // optional: for S3-compatible endpoints
	ACL             string `json:",optional"` // optional: e.g. "public-read" (leave empty if bucket enforces ownership)
}

type Config struct {
	rest.RestConf

	// JWT配置（用于中间件验证 token）
	Auth struct {
		AccessSecret  string
		AccessExpire  int64
		RefreshSecret string
		RefreshExpire int64
	}

	// Admin JWT配置（后台管理独立密钥对）
	AdminAuth struct {
		AccessSecret  string `json:",optional"`
		AccessExpire  int64  `json:",optional"`
		RefreshSecret string `json:",optional"`
		RefreshExpire int64  `json:",optional"`
	} `json:",optional"`

	// RPC服务配置 - 企业内部钱包系统
	BusinessRpc  zrpc.RpcClientConf // 业务服务（用户、账户、转账等）
	ChainRpcRpc  zrpc.RpcClientConf // 链操作服务（地址生成、交易构建、广播）
	ChainSyncRpc zrpc.RpcClientConf // 扫链服务（充值监听、交易确认）
	AdminRpc     zrpc.RpcClientConf `json:",optional"` // 后台管理服务（Admin IAM）

	// Redis配置（用于分布式限流）
	Redis struct {
		Host     string
		Port     int
		Password string
		DB       int
	}

	// 限流配置
	RateLimit struct {
		Enable      bool // 是否启用限流
		UseRedis    bool // 是否使用Redis限流（false则使用内存限流）
		GlobalRate  int  // 全局限流（每秒请求数）
		GlobalBurst int  // 全局突发容量
	}

	// 超时配置 (毫秒)
	RequestTimeout int64

	// CORS配置
	Cors struct {
		AllowOrigins     []string
		AllowMethods     []string
		AllowHeaders     []string
		ExposeHeaders    []string
		AllowCredentials bool
		MaxAge           int
	}

	// S3 上传配置（用于管理后台上传币种图标等静态资源）
	S3 S3Config `json:",optional"`

	// Geetest 配置（可选）
	Geetest GeetestConfig `json:",optional"`

	// 可信代理配置（用于安全的客户端IP提取）
	// 仅当请求来自这些CIDR范围内的代理时，才信任X-Forwarded-For/X-Real-IP标头
	TrustedProxies []string `json:",optional"`
}

// GeetestConfig holds Geetest captcha config (optional for gateway).
type GeetestConfig struct {
	Enabled    bool   `json:",optional"`
	CaptchaID  string `json:",optional"`
	CaptchaKey string `json:",optional"`
	APIServer  string `json:",optional"`
	Timeout    int64  `json:",optional"`
	FailOpen   bool   `json:",optional"`
}
