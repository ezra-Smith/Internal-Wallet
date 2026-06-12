package config

import (
	"fmt"
	"strings"

	commonDB "internalwallet/common/db"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf

	// 雪花ID配置（必须配置，范围0-1023，每个服务唯一）
	NodeID int64 `json:",optional"`

	// GORM MySQL 配置（使用通用配置）
	MySQL commonDB.MySQLConfig `json:",optional"`

	// Redis 缓存配置
	CacheRedis cache.CacheConf

	// Kafka配置
	Kafka KafkaConfig

	// Kafka consumer configuration (used for address monitor events, etc.).
	KafkaConsumer KafkaConsumerConfig `json:",optional"`

	// Signer服务配置（用于获取监控地址）- 已废弃，改用 Admin
	Signer zrpc.RpcClientConf

	// Admin服务配置（用于获取监控地址列表）
	Admin zrpc.RpcClientConf

	// 区块链服务商配置
	Providers struct {
		Ethereum   ProviderConfig   `json:"ethereum"`
		QuickNode  ProviderConfig   `json:"quickNode"`
		Infura     ProviderConfig   `json:"infura"`
		Alchemy    ProviderConfig   `json:"alchemy"`
		BSC        ProviderConfig   `json:"bsc"`
		Tron       ProviderConfig   `json:"tron"`
		SelfHosted []ProviderConfig `json:"selfHosted"` // 自建节点服务商配置
		Custom     []ProviderConfig `json:"custom"`
	}

	// 链配置
	Chains struct {
		Ethereum ChainConfig `json:"Ethereum"`
		BSC      ChainConfig `json:"BSC"`
		Tron     ChainConfig `json:"TRON"`
	}

	// 同步配置
	Sync struct {
		Enabled                bool `json:",default=false"` // 默认关闭区块同步（自建节点场景）
		MaxConcurrentSyncs     int  `json:",default=10"`
		DefaultRetryCount      int  `json:",default=3"`
		HealthCheckInterval    int  `json:",default=30"`  // 秒
		FailoverThreshold      int  `json:",default=3"`   // 连续失败次数阈值
		ProviderSwitchCooldown int  `json:",default=60"`  // 服务商切换冷却时间（秒）
		BatchSize              int  `json:",default=100"` // 批量处理大小
		MaxRetryDelay          int  `json:",default=60"`  // 最大重试延迟（秒）
		BatchUpdateInterval    int  `json:",default=30"`  // 批量更新间隔（秒）
		InitialDelay           int  `json:",default=5"`   // 启动延迟（秒）
		TaskRetryInterval      int  `json:",default=5"`   // 任务重试间隔（秒）
	}

	// 监控配置
	Monitoring struct {
		MetricsEnabled          bool                           `json:",default=true"`
		PrometheusPort          int                            `json:",default=9090"`
		AddressPoolMonitoring   bool                           `json:",default=false"` // 地址池监控开关，默认关闭
		AddressRegistry         AddressRegistryConfig          `json:",optional"`      // 地址注册表刷新/来源配置
		AddressMonitoring       *AddressMonitoringConfig       `json:",optional"`      // 地址监控配置
		BalanceValidation       *BalanceValidationConfig       `json:",optional"`      // 余额校验配置
		TransactionConfirmation *TransactionConfirmationConfig `json:",optional"`      // 交易确认管理器配置
	}

	// 存储配置
	Storage struct {
		// 区块数据存储配置，默认关闭（自建节点无需存储）
		StoreBlockData bool `json:",default=false"`
		// 区块数据保留天数，仅当StoreBlockData=true时有效
		BlockDataRetentionDays int `json:",default=7"`
	}
}

func (c *Config) Validate() error {
	if c == nil {
		return fmt.Errorf("config is nil")
	}

	if c.NodeID < 0 || c.NodeID > 1023 {
		return fmt.Errorf("invalid NodeID=%d (expected 0..1023)", c.NodeID)
	}

	if len(c.CacheRedis) == 0 || strings.TrimSpace(c.CacheRedis[0].Host) == "" {
		return fmt.Errorf("CacheRedis[0].Host is required")
	}

	if c.Sync.MaxConcurrentSyncs <= 0 {
		return fmt.Errorf("Sync.MaxConcurrentSyncs must be > 0")
	}
	if c.Sync.BatchSize <= 0 {
		return fmt.Errorf("Sync.BatchSize must be > 0")
	}
	if c.Sync.HealthCheckInterval <= 0 {
		return fmt.Errorf("Sync.HealthCheckInterval must be > 0")
	}
	if c.Sync.FailoverThreshold <= 0 {
		return fmt.Errorf("Sync.FailoverThreshold must be > 0")
	}

	if !c.Monitoring.AddressPoolMonitoring {
		return fmt.Errorf("Monitoring.AddressPoolMonitoring must be true (full-chain scanning mode is removed)")
	}

	kafkaRequired := !strings.EqualFold(c.Mode, "dev") && !strings.EqualFold(c.Mode, "test")
	if kafkaRequired && len(c.Kafka.Brokers) == 0 {
		return fmt.Errorf("Kafka.Brokers is required when Mode=%s", c.Mode)
	}
	if len(c.Kafka.Brokers) > 0 {
		if c.Kafka.UseAsync {
			return fmt.Errorf("Kafka.UseAsync=true is not supported (chainsync marks DB rows as sent before Kafka ack); use sync producer")
		}
		if strings.TrimSpace(c.Kafka.Topics.TransactionDeposit) == "" {
			return fmt.Errorf("Kafka.Topics.TransactionDeposit is required when Kafka is enabled")
		}
		if strings.TrimSpace(c.Kafka.Topics.TransactionWeb3) == "" {
			return fmt.Errorf("Kafka.Topics.TransactionWeb3 is required when Kafka is enabled")
		}
		if strings.TrimSpace(c.Kafka.Topics.TransactionVault) == "" {
			return fmt.Errorf("Kafka.Topics.TransactionVault is required when Kafka is enabled")
		}
		if strings.TrimSpace(c.Kafka.Topics.BalanceChange) == "" {
			return fmt.Errorf("Kafka.Topics.BalanceChange is required when Kafka is enabled")
		}
	}

	hasProvider := func(pc ProviderConfig) bool {
		return pc.Enabled && strings.TrimSpace(pc.Endpoint) != ""
	}
	hasSelfHosted := func(types ...string) bool {
		want := make(map[string]struct{}, len(types))
		for _, t := range types {
			want[strings.ToLower(strings.TrimSpace(t))] = struct{}{}
		}
		for _, sh := range c.Providers.SelfHosted {
			if !sh.Enabled || strings.TrimSpace(sh.Endpoint) == "" {
				continue
			}
			if _, ok := want[strings.ToLower(strings.TrimSpace(sh.Type))]; ok {
				return true
			}
		}
		return false
	}

	if c.Chains.Ethereum.Enabled {
		hasEthProvider := hasProvider(c.Providers.Ethereum) ||
			hasProvider(c.Providers.QuickNode) ||
			hasProvider(c.Providers.Infura) ||
			hasProvider(c.Providers.Alchemy) ||
			hasSelfHosted("ethereum", "eth")
		if !hasEthProvider {
			return fmt.Errorf("no providers configured for enabled chain Ethereum")
		}
	}

	if c.Chains.BSC.Enabled {
		hasBscProvider := hasProvider(c.Providers.BSC) || hasSelfHosted("bsc")
		if !hasBscProvider {
			return fmt.Errorf("no providers configured for enabled chain BSC")
		}
	}

	if c.Chains.Tron.Enabled {
		hasTronProvider := hasProvider(c.Providers.Tron) || hasSelfHosted("tron", "trc20")
		if !hasTronProvider {
			return fmt.Errorf("no providers configured for enabled chain Tron")
		}
	}

	return nil
}

// AddressRegistryConfig controls how chainsync maintains monitored address sets.
type AddressRegistryConfig struct {
	// Full refresh interval (seconds). Full refresh remains the reconciliation source-of-truth.
	RefreshIntervalSeconds int `json:",default=60"`
	// Whether to include web3_user_addresses in monitoring set.
	MonitorWeb3Addresses *bool `json:",optional"`
	// Whether web3 addresses should be treated as internal (usually false).
	TreatWeb3AsInternal bool `json:",default=false"`
}

// ProviderConfig 服务商配置
type ProviderConfig struct {
	Enabled     bool              `json:",default=true"`
	Name        string            `json:",required"`
	Endpoint    string            `json:",required"`
	APIKey      string            `json:",optional"`
	SecretKey   string            `json:",optional"`
	Weight      float64           `json:",default=1.0"` // 权重，用于负载均衡
	Timeout     int               `json:",default=30"`  // 请求超时时间（秒）
	RateLimit   int               `json:",default=100"` // 速率限制（请求/秒）
	MaxRetries  int               `json:",default=3"`   // 最大重试次数
	RetryDelay  int               `json:",default=1"`   // 重试延迟（秒）
	Headers     map[string]string `json:",optional"`    // 自定义请求头
	HealthCheck HealthCheckConfig `json:"healthCheck"`  // 健康检查配置

	// 自建节点专用配置 - 基于RPC代理服务
	Type string `json:",optional"` // 节点类型: "ethereum", "bsc", "tron"
}

// HealthCheckConfig 健康检查配置
type HealthCheckConfig struct {
	Enabled      bool   `json:",default=true"`
	Endpoint     string `json:",optional"`   // 健康检查端点，不填则使用默认
	Interval     int    `json:",default=30"` // 检查间隔（秒）
	Timeout      int    `json:",default=10"` // 检查超时（秒）
	FailureCount int    `json:",default=3"`  // 失败次数阈值
}

// ChainConfig 链配置
type ChainConfig struct {
	Enabled           bool              `json:",default=true"`
	Name              string            `json:",required"`
	ChainType         string            `json:",required"`   // ethereum, bsc, tron
	ChainID           int64             `json:",required"`   // 链ID
	BlockLookback     uint64            `json:",default=20"` // 从最新区块往前回溯的区块数
	Confirmations     uint64            `json:",default=12"` // 确认数
	BlockTime         int               `json:",optional"`   // 区块时间（秒）
	DefaultProviders  []string          `json:",optional"`   // 默认服务商列表
	ContractAddresses map[string]string `json:",optional"`   // 常用合约地址
	GasConfig         GasConfig         `json:"gasConfig"`   // Gas配置
	ScanConfig        ChainScanConfig   `json:"scanConfig"`  // 链级别扫描配置
}

// ChainScanConfig 链级别扫描配置
type ChainScanConfig struct {
	BatchSize       int `json:",default=100"` // 每批处理的区块数量
	CheckInterval   int `json:",default=5"`   // 检查间隔（秒）
	ProgressLogStep int `json:",default=100"` // 进度日志间隔
}

// GasConfig Gas相关配置
type GasConfig struct {
	GasPriceMultiplier float64 `json:",default=1.0"` // Gas价格乘数
	GasLimitMultiplier float64 `json:",default=1.1"` // Gas限制乘数
	MaxGasPrice        string  `json:",optional"`    // 最大Gas价格
}

// BalanceValidationConfig 余额校验配置
type BalanceValidationConfig struct {
	Enabled       bool `json:",default=false"` // 是否启用余额校验
	CheckInterval int  `json:",default=5"`     // 校验间隔（分钟）
	BatchSize     int  `json:",default=20"`    // 批量查询大小
	Timeout       int  `json:",default=10"`    // 请求超时（秒）
}

// AddressMonitoringConfig 地址监控配置
type AddressMonitoringConfig struct {
	CheckInterval  int `json:",default=5"`  // 新区块检查间隔（秒），基于区块的增量扫描
	InitialDelay   int `json:",default=3"`  // 启动延迟（秒）
	Timeout        int `json:",default=30"` // 请求超时（秒）
	WorkerPoolSize int `json:",default=10"` // 工作池大小
}

// TransactionConfirmationConfig 交易确认管理器配置
type TransactionConfirmationConfig struct {
	CheckInterval   int `json:",default=15"`  // 检查间隔（秒）
	InitialDelay    int `json:",default=5"`   // 启动延迟（秒）
	MaxRetries      int `json:",default=3"`   // 最大重试次数
	Timeout         int `json:",default=30"`  // 请求超时（秒）
	CleanupInterval int `json:",default=300"` // 清理过期交易间隔（秒）
}

// KafkaConfig Kafka配置
type KafkaConfig struct {
	Brokers     []string `json:",required"`          // Kafka broker列表
	Username    string   `json:",optional"`          // 用户名
	Password    string   `json:",optional"`          // 密码
	Security    string   `json:",default=PLAINTEXT"` // 安全协议
	SASLMech    string   `json:",optional"`          // SASL机制
	ClientID    string   `json:",optional"`          // 客户端ID
	Compression string   `json:",optional"`          // 压缩算法
	MaxRetries  int      `json:",default=3"`         // 最大重试次数
	Timeout     int      `json:",default=30"`        // 超时时间（秒）
	UseAsync    bool     `json:",default=false"`     // 是否使用异步生产者

	// Topic 配置
	Topics KafkaTopicsConfig `json:"topics"` // Kafka主题配置
}

// KafkaTopicsConfig Kafka主题配置
type KafkaTopicsConfig struct {
	// 交易确认通知主题（按来源拆分，确保独立失败域）
	TransactionDeposit string `json:",default=wallet.transactions.confirm.deposit"`
	TransactionWeb3    string `json:",default=wallet.transactions.confirm.web3"`
	TransactionVault   string `json:",default=wallet.transactions.confirm.vault"` // includes company/vault sources
	TransactionManual  string `json:",default=wallet.transactions.confirm.manual"`
	TransactionUnknown string `json:",default=wallet.transactions.confirm.unknown"`
	// 余额变动主题
	BalanceChange string `json:",default=wallet.balance.changes"` // 余额变动通知主题
	// 地址监控事件主题（用于地址池增量更新）
	AddressMonitorEvent string `json:",default=wallet.address.monitor.events"`
}

// KafkaConsumerConfig defines consumer settings. If Brokers is empty, chainsync will reuse Kafka.Brokers.
type KafkaConsumerConfig struct {
	Enabled           bool     `json:",default=true"`
	Brokers           []string `json:",optional"`
	GroupID           string   `json:",optional,default=chainsync-address-monitor"`
	Username          string   `json:",optional"`
	Password          string   `json:",optional"`
	Security          string   `json:",optional,default=PLAINTEXT"` // PLAINTEXT, SASL_PLAINTEXT, SASL_SSL, SSL
	SASLMech          string   `json:",optional,default=PLAIN"`     // PLAIN, SCRAM-SHA-256, SCRAM-SHA-512
	SessionTimeout    int      `json:",optional,default=30"`        // seconds
	RebalanceTimeout  int      `json:",optional,default=60"`        // seconds
	MaxProcessingTime int      `json:",optional,default=300"`       // seconds
}
