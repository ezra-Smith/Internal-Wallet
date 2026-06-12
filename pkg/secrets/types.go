package secrets

import (
	"fmt"
	"time"
)

// SecretConfig 总配置结构（对应 AWS Secrets Manager 中的 JSON）
type SecretConfig struct {
	Version     string    `json:"version"`
	Environment string    `json:"environment"`
	LastUpdated time.Time `json:"last_updated"`

	Database      DatabaseConfig     `json:"database"`
	Cache         CacheConfig        `json:"cache"`
	Messaging     MessagingConfig    `json:"messaging"`
	Security      SecurityConfig     `json:"security"`
	Notifications NotificationConfig `json:"notifications"`
	Monitoring    MonitoringConfig   `json:"monitoring"`

	// New fields for additional secrets
	Swap SwapConfig      `json:"swap"`
	SMS  SMSSecretConfig `json:"sms"`
	S3   S3ConfigSecrets `json:"s3"`
	// Geetest captcha (optional)
	Geetest GeetestSecretConfig `json:"geetest"`

	// Service-specific secrets (optional)
	ChainRPC      ChainRPCSecretConfig      `json:"chainrpc"`
	ChainSync     ChainSyncSecretConfig     `json:"chainsync"`
	Consolidation ConsolidationSecretConfig `json:"consolidation"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	MySQL MySQLConfig `json:"mysql"`
}

type MySQLConfig struct {
	Host           string `json:"host"`
	Port           int    `json:"port"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	Database       string `json:"database"`
	MaxConnections int    `json:"max_connections"`
}

// CacheConfig 缓存配置
type CacheConfig struct {
	Redis RedisConfig `json:"redis"`
}

type RedisConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Password string `json:"password"`
	DB       int    `json:"db"`
}

// Address 返回 Redis 地址（host:port 格式）
// 用于 Go-Zero CacheRedis 配置（需要 "host:port" 格式）
func (r *RedisConfig) Address() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

// MessagingConfig 消息队列配置
type MessagingConfig struct {
	Kafka KafkaConfig `json:"kafka"`
}

type KafkaConfig struct {
	Brokers  []string `json:"brokers"`
	Username string   `json:"username"`
	Password string   `json:"password"`
}

// SecurityConfig 安全配置
type SecurityConfig struct {
	Signer   SignerConfig `json:"signer"`
	JWT      JWTConfig    `json:"jwt"`
	AdminJWT JWTConfig    `json:"admin_jwt"`
}

type SignerConfig struct {
	EncryptionPassword string `json:"encryption_password"`
	Notes              string `json:"notes,omitempty"`
}

type JWTConfig struct {
	AccessSecret        string `json:"access_secret"`
	RefreshSecret       string `json:"refresh_secret"`
	AccessExpireMinutes int    `json:"access_expire_minutes"`
	RefreshExpireDays   int    `json:"refresh_expire_days"`
}

// NotificationConfig 通知配置
type NotificationConfig struct {
	SMTP  SMTPConfig  `json:"smtp"`
	JPush JPushConfig `json:"jpush"`
}

type JPushConfig struct {
	AppKey         string `json:"app_key"`
	MasterSecret   string `json:"master_secret"`
	ApnsProduction bool   `json:"apns_production"` // iOS推送环境：false=开发 true=生产
}

type SMTPConfig struct {
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	FromAddress string `json:"from_address"`
	FromName    string `json:"from_name"`
}

// MonitoringConfig 监控配置
type MonitoringConfig struct {
	GrafanaAdminPassword string `json:"grafana_admin_password"`
}

// SwapConfig holds swap service credentials
type SwapConfig struct {
	ApiKeyPepper        string `json:"api_key_pepper"`
	AdminToken          string `json:"admin_token"`
	AdminTokenSHA256    string `json:"admin_token_sha256"`
	ProviderType        string `json:"provider_type"`
	OKXApiKey           string `json:"okx_api_key"`
	OKXSecretKey        string `json:"okx_secret_key"`
	OKXPassphrase       string `json:"okx_passphrase"`
	OneInchApiKey       string `json:"oneinch_api_key"`
	BusinessSwapApiKey  string `json:"business_swap_api_key"`
	AdminSwapAdminToken string `json:"admin_swap_admin_token"`
}

// SMSSecretConfig holds SMS provider credentials
type SMSSecretConfig struct {
	// International SMS
	InternationalURL       string `json:"international_url"`
	InternationalAppKey    string `json:"international_app_key"`
	InternationalAppSecret string `json:"international_app_secret"`
	InternationalAppCode   string `json:"international_app_code"`
	// Domestic SMS (Fengniao)
	DomesticURL        string `json:"domestic_url"`
	DomesticMercId     string `json:"domestic_merc_id"`
	DomesticSecret     string `json:"domestic_secret"`
	DomesticTemplateId string `json:"domestic_template_id"`
	// Signature
	Sign string `json:"sign"`
}

// S3ConfigSecrets holds S3 configuration (not credentials - use IRSA)
type S3ConfigSecrets struct {
	Region        string `json:"region"`
	Bucket        string `json:"bucket"`
	Endpoint      string `json:"endpoint"`
	PublicBaseURL string `json:"public_base_url"`
	Prefix        string `json:"prefix"`
	UsePathStyle  bool   `json:"use_path_style"`
	ACL           string `json:"acl"`
}

// GeetestSecretConfig holds Geetest captcha credentials
type GeetestSecretConfig struct {
	Enabled    bool   `json:"enabled"`
	CaptchaID  string `json:"captcha_id"`
	CaptchaKey string `json:"captcha_key"`
	APIServer  string `json:"api_server"`
	Timeout    int64  `json:"timeout_ms"`
	FailOpen   bool   `json:"fail_open"`
}

// ChainRPCSecretConfig holds API keys for chainrpc upstreams (optional).
type ChainRPCSecretConfig struct {
	EthRpcApiKey string `json:"eth_rpc_api_key"`
	BscRpcApiKey string `json:"bsc_rpc_api_key"`
	TronApiKey   string `json:"tron_api_key"`
}

// ChainSyncSecretConfig holds provider API keys for chainsync (optional).
type ChainSyncSecretConfig struct {
	EthereumApiKey string `json:"ethereum_api_key"`
	BscApiKey      string `json:"bsc_api_key"`
	TronApiKey     string `json:"tron_api_key"`

	QuickNodeApiKey  string `json:"quicknode_api_key"`
	InfuraApiKey     string `json:"infura_api_key"`
	AlchemyApiKey    string `json:"alchemy_api_key"`
	SelfHostedApiKey string `json:"selfhosted_api_key"`
}

// ConsolidationSecretConfig holds consolidation service secrets (optional).
type ConsolidationSecretConfig struct {
	ITRXApiEndpoint string `json:"itrx_api_endpoint"`
	ITRXApiKey      string `json:"itrx_api_key"`
	ITRXApiSecret   string `json:"itrx_api_secret"`
}
