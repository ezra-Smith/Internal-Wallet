package config

import (
	commonDB "internalwallet/common/db"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf

	// 雪花ID配置（必须配置，范围0-1023，每个服务唯一）
	NodeID int64 `json:",optional"`

	// Redis 缓存配置（用于会话、幂等、2FA临时token等）
	CacheRedis cache.CacheConf `json:",optional"`

	// GORM MySQL/MariaDB 配置（统一使用 common/db）
	MySQL commonDB.MySQLConfig `json:",optional"`

	// RPC clients (optional)
	ChainSyncRpc     zrpc.RpcClientConf `json:",optional"` // 扫链服务（Vault 同步等）
	ChainRpc         zrpc.RpcClientConf `json:",optional"` // 链上转账服务（提现放币等）
	SignerRpc        zrpc.RpcClientConf `json:",optional"` // 签名服务（地址生成等）
	AccountingRpc    zrpc.RpcClientConf `json:",optional"` // Accounting 服务（账本/余额唯一写入口）
	SwapRpc          zrpc.RpcClientConf `json:",optional"` // Swap 服务（Swap配置管理）
	ConsolidationRpc zrpc.RpcClientConf `json:",optional"` // Consolidation 服务（归集工作流）

	// Kafka producer (optional). Used to publish address-monitor events for chainsync.
	Kafka struct {
		Brokers     []string `json:",optional"`                   // Kafka broker 地址列表
		Username    string   `json:",optional"`                   // SASL 用户名
		Password    string   `json:",optional"`                   // SASL 密码
		Security    string   `json:",optional,default=PLAINTEXT"` // PLAINTEXT, SASL_PLAINTEXT, SASL_SSL, SSL
		SASLMech    string   `json:",optional,default=PLAIN"`     // PLAIN, SCRAM-SHA-256, SCRAM-SHA-512
		ClientID    string   `json:",optional,default=admin-producer"`
		Compression string   `json:",optional,default=gzip"`
		MaxRetries  int      `json:",optional,default=3"`
		Timeout     int      `json:",optional,default=30"` // seconds
		UseAsync    bool     `json:",optional,default=false"`
		Topics      struct {
			AddressMonitorEvent string `json:",optional,default=wallet.address.monitor.events"`
		} `json:"Topics,optional"`
	} `json:"Kafka,optional"`

	// SMTP 邮件配置（用于管理员创建/重置密码邮件等）
	Email struct {
		SMTPHost           string `json:",optional"`
		SMTPPort           string `json:",optional"`
		FromAddress        string `json:",optional"`
		FromPassword       string `json:",optional"`
		FromName           string `json:",optional"`
		InsecureSkipVerify bool   `json:",optional"`

		// 邮件模板配置
		FreezeAccountURL string `json:",optional"` // 冻结账号页面 URL
		SupportEmail     string `json:",optional"` // 客服邮箱
		CompanyName      string `json:",optional"` // 公司名称

		// S3 图片资源 URL
		LogoURL           string `json:",optional"` // Zink Logo
		SocialXURL        string `json:",optional"` // X (Twitter) 图标
		SocialTelegramURL string `json:",optional"` // Telegram 图标
		SocialTikTokURL   string `json:",optional"` // TikTok 图标
		SocialLinkedInURL string `json:",optional"` // LinkedIn 图标
		SocialFacebookURL string `json:",optional"` // Facebook 图标
		SocialRedditURL   string `json:",optional"` // Reddit 图标
		AppGooglePlayURL  string `json:",optional"` // Google Play 按钮
		AppAppStoreURL    string `json:",optional"` // App Store 按钮
	} `json:"Email,optional"`

	// Admin JWT（独立密钥对）
	JWT struct {
		AccessSecret  string
		AccessExpire  int64
		RefreshSecret string
		RefreshExpire int64
	} `json:"Jwt,optional"`

	// 安全策略（与 SRD 对齐，支持按需扩展）
	Security struct {
		SessionTimeoutMinutes    int32 `json:",default=480"`
		MaxLoginAttempts         int32 `json:",default=5"`
		LockoutDurationMinutes   int32 `json:",default=30"`
		PasswordExpiryDays       int32 `json:",default=90"`
		PasswordMinLength        int32 `json:",default=10"`
		Require2FA               bool  `json:",default=true"`
		RefreshWindowSeconds     int64 `json:",default=7200"` // 2小时
		TwoFATempTokenTTLSeconds int64 `json:",default=300"`  // 5分钟
	} `json:"Security,optional"`

	// 是否启用 AutoMigrate（仅开发/测试建议开启）
	AutoMigrate bool `json:",optional"`

	// 后台任务：转账批次执行（内部账本加款）
	TransferBatchWorker struct {
		Enabled                 bool  `json:",default=false"`
		PollIntervalSeconds     int64 `json:",default=2"`
		BatchSize               int   `json:",default=20"`
		ItemTimeoutSeconds      int64 `json:",default=15"`
		ProcessingLeaseSeconds  int64 `json:",default=600"` // 超时回收 processing 明细
		MinRetryIntervalSeconds int64 `json:",default=3"`   // retrying 明细最小重试间隔
	} `json:"TransferBatchWorker,optional"`

	// 后台任务：金库余额定时同步
	VaultBalanceSyncScheduler struct {
		Enabled             bool  `json:",default=false"`
		SyncIntervalSeconds int64 `json:",default=300"` // 默认5分钟（300秒）
	} `json:"VaultBalanceSyncScheduler,optional"`

	// 后台任务：金库余额Telegram通知
	VaultBalanceNotification struct {
		Enabled                     bool  `json:",default=false"`
		NotificationIntervalSeconds int64 `json:",default=600"` // 默认10分钟（600秒），可配置为600-1800秒
	} `json:"VaultBalanceNotification,optional"`

	// Swap 服务授权配置（用于 Admin 调用 Swap 服务）
	SwapAuth struct {
		AdminToken string `json:",optional"` // Swap Admin Token（用于调用 Swap 管理接口）
	} `json:"SwapAuth,optional"`
}
