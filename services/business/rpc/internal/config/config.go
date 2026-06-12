package config

import (
	commonDB "internalwallet/common/db"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/trace"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf

	// 雪花ID配置
	NodeID int64 `json:",optional"`

	// sqlx 数据源
	DataSource string

	// Redis 缓存配置
	CacheRedis cache.CacheConf

	// GORM MySQL 配置
	MySQL commonDB.MySQLConfig `json:",optional"`

	// 链路追踪配置
	TraceConf trace.Config `json:",optional"`

	// RPC客户端配置
	SignerRpc       zrpc.RpcClientConf `json:",optional"` // 签名服务RPC配置
	ChainRpc        zrpc.RpcClientConf `json:",optional"`
	ChainSyncRpc    zrpc.RpcClientConf `json:",optional"` // ChainSync 扫链服务 RPC 配置
	SwapRpc         zrpc.RpcClientConf `json:",optional"` // Swap 微服务 RPC 配置
	AdminRpc        zrpc.RpcClientConf `json:",optional"` // Admin 服务RPC配置
	AccountingRpc   zrpc.RpcClientConf `json:",optional"` // Accounting 服务RPC配置（账本/余额唯一写入口）
	NotificationRpc zrpc.RpcClientConf `json:",optional"` // Notification 服务RPC配置（推送通知）

	// Swap 微服务认证配置（用于内部调用）
	SwapAuth struct {
		ApiKey string `json:",optional"` // Swap API Key（用于 Business 调用 Swap 服务）
	} `json:"SwapAuth,optional"`
	// Google OAuth 配置
	GoogleOauth GoogleOauthConfig `json:",optional"`

	// 全局请求超时（毫秒）
	RequestTimeoutMillis int64 `json:",optional"`

	// 数据库操作超时（毫秒）
	DBTimeoutMillis int64 `json:",optional"`

	// JWT配置（用于生成登录后的访问令牌）
	JWT struct {
		AccessSecret  string
		AccessExpire  int64
		RefreshSecret string
		RefreshExpire int64
	} `json:"Jwt,optional"`

	// 验证码配置
	Code struct {
		Length        int32  `json:",optional"`
		ExpireSeconds int32  `json:",optional"`
		BypassCode    string `json:",optional"`
	} `json:"Code,optional"`

	// Geetest 配置
	Geetest GeetestConfig `json:"Geetest,optional"`

	// 签名服务地址生成默认参数
	Signer struct {
		SeedId    string   `json:",optional"`
		Chains    []string `json:",optional"`
		Requester string   `json:",optional"`
	} `json:"Signer,optional"`

	// 支持的国家/地区区号（用于手机号相关接口校验）
	CountryCodes struct {
		Supported []string `json:",optional"`
	} `json:"CountryCodes,optional"`

	// 邮件 SMTP 配置
	Email struct {
		SMTPHost           string `json:",optional"`
		SMTPPort           string `json:",optional,default=587"`
		FromAddress        string `json:",optional"`
		FromPassword       string `json:",optional"`
		FromName           string `json:",optional,default=Zink Wallet"`
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

	// Kafka 消费者配置（用于消费 chainsync 的交易确认和余额变动消息）
	KafkaConsumer struct {
		Brokers           []string            `json:",optional"`                   // Kafka broker 地址列表
		GroupID           string              `json:",optional"`                   // 消费者组ID
		Username          string              `json:",optional"`                   // SASL 用户名
		Password          string              `json:",optional"`                   // SASL 密码
		Security          string              `json:",optional,default=PLAINTEXT"` // 安全协议: PLAINTEXT, SASL_PLAINTEXT, SASL_SSL, SSL
		SASLMech          string              `json:",optional,default=PLAIN"`     // SASL机制: PLAIN, SCRAM-SHA-256, SCRAM-SHA-512
		Topics            KafkaConsumerTopics `json:",optional"`                   // 订阅的主题
		MaxRetries        int                 `json:",optional,default=3"`         // 最大重试次数
		SessionTimeout    int                 `json:",optional,default=30"`        // 会话超时（秒）
		RebalanceTimeout  int                 `json:",optional,default=60"`        // 重平衡超时（秒）
		MaxProcessingTime int                 `json:",optional,default=300"`       // 最大处理时间（秒）
		Workers           int                 `json:",optional,default=5"`         // 并发处理 worker 数量
	} `json:"KafkaConsumer,optional"`

	// Kafka producer (optional). Used to publish address-monitor events for chainsync incremental updates.
	KafkaProducer struct {
		Brokers     []string `json:",optional"`                   // Kafka broker 地址列表
		Username    string   `json:",optional"`                   // SASL 用户名
		Password    string   `json:",optional"`                   // SASL 密码
		Security    string   `json:",optional,default=PLAINTEXT"` // PLAINTEXT, SASL_PLAINTEXT, SASL_SSL, SSL
		SASLMech    string   `json:",optional,default=PLAIN"`     // PLAIN, SCRAM-SHA-256, SCRAM-SHA-512
		ClientID    string   `json:",optional,default=business-producer"`
		Compression string   `json:",optional,default=gzip"`
		MaxRetries  int      `json:",optional,default=3"`
		Timeout     int      `json:",optional,default=30"` // seconds
		UseAsync    bool     `json:",optional,default=false"`
		Topics      struct {
			AddressMonitorEvent string `json:",optional,default=wallet.address.monitor.events"`
		} `json:"Topics,optional"`
	} `json:"KafkaProducer,optional"`

	// Web3 broadcast placeholder reconcile worker (best-effort on-chain status refresh).
	Web3PlaceholderReconcile Web3PlaceholderReconcileConfig `json:"Web3PlaceholderReconcile,optional"`
}

// Web3PlaceholderReconcileConfig controls the scheduled worker that refreshes pending broadcast placeholders
// (web3_balance_changes with event_index=-1, status=pending).
type Web3PlaceholderReconcileConfig struct {
	Enabled bool `json:",optional"`

	IntervalSeconds     int64 `json:",optional,default=5"`  // scan interval
	BatchSize           int   `json:",optional,default=50"` // records per tick
	Concurrency         int   `json:",optional,default=5"`  // max concurrent chain calls
	MinRecheckSeconds   int64 `json:",optional,default=15"` // do not recheck same record too frequently (via updated_at)
	LookbackHours       int64 `json:",optional,default=48"` // only scan recent placeholders
	FailAfterHours      int64 `json:",optional,default=24"` // mark failed if still not found after this age (only on "not found" responses)
	ChainTimeoutSeconds int64 `json:",optional,default=5"`  // per-chain call timeout
}

// KafkaConsumerTopics Kafka 消费者主题配置
type KafkaConsumerTopics struct {
	// 交易确认主题（按来源拆分，确保独立失败域）
	TransactionDeposit string `json:",optional,default=wallet.transactions.confirm.deposit"`
	TransactionWeb3    string `json:",optional,default=wallet.transactions.confirm.web3"`
	TransactionVault   string `json:",optional,default=wallet.transactions.confirm.vault"` // includes company/vault sources
	TransactionManual  string `json:",optional,default=wallet.transactions.confirm.manual"`
	TransactionUnknown string `json:",optional,default=wallet.transactions.confirm.unknown"`
	// 余额变动主题
	BalanceChange string `json:",optional,default=wallet.balance.changes"` // 余额变动主题
}

type GoogleOauthConfig struct {
	ClientId     string   `json:"ClientId"`
	ClientSecret string   `json:"ClientSecret"`
	RedirectUrl  string   `json:"RedirectUrl"`
	Scopes       []string `json:"Scopes"`
	State        string   `json:"State"`
}

type GeetestConfig struct {
	Enabled    bool   `json:",optional"`
	CaptchaID  string `json:",optional"`
	CaptchaKey string `json:",optional"`
	APIServer  string `json:",optional"`
	Timeout    int64  `json:",optional"`
	FailOpen   bool   `json:",optional"`
}
