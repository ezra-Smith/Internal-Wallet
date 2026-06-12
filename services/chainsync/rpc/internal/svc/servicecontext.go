package svc

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"

	"internalwallet/common/cache"
	commonDB "internalwallet/common/db"
	"internalwallet/common/utils"
	"internalwallet/proto/pb"
	"internalwallet/services/chainsync/rpc/internal/config"
	chainModel "internalwallet/services/chainsync/rpc/internal/model"
	"internalwallet/services/chainsync/rpc/internal/provider"
	"internalwallet/services/chainsync/rpc/internal/provider/evm"
	"internalwallet/services/chainsync/rpc/internal/provider/selfhosted"
	"internalwallet/services/chainsync/rpc/internal/provider/tron"
	"internalwallet/services/chainsync/rpc/internal/repository"
)

type ServiceContext struct {
	Config *config.Config

	// GORM
	DB *gorm.DB

	// Redis
	RedisClient       *redis.Client
	ChainCache        *cache.RedisCache
	RedisCacheManager *RedisCacheManager

	// Repository层
	BlockRepository              repository.BlockRepository
	TransactionRepository        repository.TransactionRepository
	AddressMonitorRepository     repository.AddressMonitorRepository
	UnconfirmedTransactionRepo   repository.UnconfirmedTransactionRepository
	KafkaFailedMessageRepository repository.KafkaFailedMessageRepository

	// 服务组件
	ProviderPool              *provider.Pool
	HeadTracker               *HeadTracker
	AddressMonitor            *AddressMonitor
	TransactionConfirmManager *TransactionConfirmManager
	BalanceValidator          *BalanceValidator      // 余额校验器
	KafkaProducer             KafkaProducerInterface // Kafka生产者
	AddressLoader             *AddressLoader         // 地址加载器
	KafkaRetryQueue           *KafkaRetryQueue       // Kafka重试队列

	startOnce sync.Once
	stopOnce  sync.Once
}

func NewServiceContext(c config.Config) *ServiceContext {
	// 初始化雪花ID生成器（必须在服务启动时初始化）
	if err := utils.Init(c.NodeID); err != nil {
		logx.Severef("Failed to initialize snowflake ID generator: %v", err)
	}
	logx.Infof("Snowflake ID generator initialized with NodeID: %d", c.NodeID)

	// 创建 Redis 客户端
	redisClient := redis.NewClient(&redis.Options{
		Addr:         c.CacheRedis[0].Host,
		Password:     c.CacheRedis[0].Pass,
		DB:           0,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     10,
		MinIdleConns: 3,
	})

	// 测试 Redis 连接
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		logx.Errorf("Failed to connect to Redis: %v", err)
	} else {
		logx.Info("✓ Connected to Redis for chainsync service")
	}

	// 创建缓存实例
	chainCache := cache.NewRedisCache(redisClient, "chainsync")

	// 创建Redis缓存管理器
	redisCacheManager := NewRedisCacheManager(redisClient, "chainsync")

	// 初始化 GORM
	var gormDB *gorm.DB
	if c.MySQL.Database != "" {
		var err error
		gormDB, err = commonDB.InitGorm(c.MySQL)
		if err != nil {
			logx.Errorf("Failed to initialize GORM: %v", err)
		} else {
			logx.Info("✓ GORM initialized successfully")

			// 自动迁移数据库表,等实际上线以后可以通过一次性全量迁移来完成，然后注释这些代码
			err = autoMigrate(gormDB)
			if err != nil {
				logx.Errorf("Failed to auto migrate database: %v", err)
			} else {
				logx.Info("✓ Database migration completed")
			}
			//-------------------------------------
		}
	}

	// 初始化 Repository
	var blockRepo repository.BlockRepository
	var txRepo repository.TransactionRepository
	var monitorRepo repository.AddressMonitorRepository
	var unconfirmedTxRepo repository.UnconfirmedTransactionRepository
	var kafkaFailedMsgRepo repository.KafkaFailedMessageRepository

	if gormDB != nil {
		blockRepo = repository.NewBlockRepository(gormDB)
		txRepo = repository.NewTransactionRepository(gormDB)
		monitorRepo = repository.NewAddressMonitorRepository(gormDB)
		unconfirmedTxRepo = repository.NewUnconfirmedTransactionRepository(gormDB)
		kafkaFailedMsgRepo = repository.NewKafkaFailedMessageRepository(gormDB)
		logx.Info("✓ Repositories initialized with GORM")
	} else {
		logx.Info("Repositories not initialized (GORM not available)")
	}

	// 初始化统一服务商池
	providerPool := provider.NewPool(&provider.PoolConfig{
		HealthCheckInterval:   time.Duration(c.Sync.HealthCheckInterval) * time.Second,
		FailoverThreshold:     c.Sync.FailoverThreshold,
		RecoveryCheckInterval: 5 * time.Minute,
		MaxConcurrentRequests: c.Sync.BatchSize * 2,
	})

	// 初始化所有服务商
	initProviders(providerPool, &c)

	// HeadTracker tracks per-chain heads (latest block numbers).
	headTracker := NewHeadTracker(providerPool, redisCacheManager, &c)

	// 初始化Kafka生产者
	//
	// 本地开发(dev/test)环境可能不启动 Kafka（例如 dev-admin.sh 的 docker infra 里不包含 Kafka）。
	// 这时 chainsync 仍应能启动：只是不发送 Kafka 通知。
	logx.Infof("🔧 Initializing Kafka producer...")
	logx.Infof("📋 Kafka configuration - Brokers: %v, Security: %s, ClientID: %s",
		c.Kafka.Brokers, c.Kafka.Security, c.Kafka.ClientID)

	var kafkaProducer KafkaProducerInterface
	kafkaRequired := !strings.EqualFold(c.Mode, "dev") && !strings.EqualFold(c.Mode, "test")

	if len(c.Kafka.Brokers) == 0 {
		if kafkaRequired {
			logx.Error("❌ Kafka brokers not configured (empty list)")
			logx.Error("💡 Please configure Kafka.Brokers in config file")
			panic("Kafka brokers configuration is required, cannot start service without Kafka")
		}
		logx.Infof("⚠️  Kafka brokers not configured; Kafka producer disabled (mode=%s)", c.Mode)
	} else {
		logx.Infof("✓ Kafka brokers configured: %v", c.Kafka.Brokers)
		kafkaConfig := &KafkaConfig{
			Brokers:     c.Kafka.Brokers,
			Username:    c.Kafka.Username,
			Password:    c.Kafka.Password,
			Security:    c.Kafka.Security,
			SASLMech:    c.Kafka.SASLMech,
			ClientID:    c.Kafka.ClientID,
			Compression: c.Kafka.Compression,
			MaxRetries:  c.Kafka.MaxRetries,
			Timeout:     c.Kafka.Timeout,
		}

		logx.Infof("🔄 Creating Kafka producer (UseAsync: %v)...", c.Kafka.UseAsync)
		kp, err := NewSaramaKafkaProducer(kafkaConfig, c.Kafka.UseAsync)
		if err != nil {
			if kafkaRequired {
				logx.Errorf("❌ Failed to create kafka producer: %v", err)
				logx.Error("💡 Please check:")
				logx.Error("   1. Kafka brokers address is correct and accessible")
				logx.Error("   2. Kafka service is running")
				logx.Error("   3. Network connectivity between chainsync and Kafka")
				logx.Error("   4. Firewall rules allow connection to Kafka port")
				panic(fmt.Sprintf("Failed to initialize Kafka producer: %v", err))
			}
			logx.Errorf("⚠️  Failed to create kafka producer, Kafka disabled (mode=%s): %v", c.Mode, err)
		} else {
			kafkaProducer = kp
			logx.Info("✅ Kafka producer initialized successfully")
			logx.Infof("📤 Kafka topics configured - Deposit: %s, Web3: %s, Vault: %s, BalanceChange: %s",
				c.Kafka.Topics.TransactionDeposit,
				c.Kafka.Topics.TransactionWeb3,
				c.Kafka.Topics.TransactionVault,
				c.Kafka.Topics.BalanceChange,
			)
		}
	}

	// 初始化地址加载器（用于从signer服务获取监控地址）
	var addressLoader *AddressLoader
	if al, err := NewAddressLoader(&c); err != nil {
		logx.Errorf("Failed to initialize address loader: %v", err)
	} else {
		addressLoader = al
		logx.Info("✅ Address loader initialized successfully")
	}

	// 初始化地址监控器
	var addressMonitor *AddressMonitor
	// 即使没有数据库连接也要启动地址监控
	addressMonitor = NewAddressMonitor(providerPool, monitorRepo, unconfirmedTxRepo, headTracker, redisCacheManager, kafkaProducer, &c, addressLoader)

	// 初始化Kafka重试队列
	var kafkaRetryQueue *KafkaRetryQueue
	if kafkaFailedMsgRepo != nil && kafkaProducer != nil {
		kafkaRetryQueue = NewKafkaRetryQueue(kafkaFailedMsgRepo, kafkaProducer)
		logx.Info("✅ Kafka retry queue initialized")
	}

	// 初始化交易确认管理器
	var confirmManager *TransactionConfirmManager
	if unconfirmedTxRepo != nil {
		confirmManager = NewTransactionConfirmManagerWithConfig(unconfirmedTxRepo, providerPool, headTracker, kafkaProducer, &c, kafkaRetryQueue)
	}

	// 初始化余额校验器
	var balanceValidator *BalanceValidator
	if unconfirmedTxRepo != nil {
		balanceValidator = NewBalanceValidator(providerPool, monitorRepo, redisCacheManager, kafkaProducer)
	}

	sc := &ServiceContext{
		Config:                       &c,
		DB:                           gormDB,
		RedisClient:                  redisClient,
		ChainCache:                   chainCache,
		RedisCacheManager:            redisCacheManager,
		BlockRepository:              blockRepo,
		TransactionRepository:        txRepo,
		AddressMonitorRepository:     monitorRepo,
		UnconfirmedTransactionRepo:   unconfirmedTxRepo,
		KafkaFailedMessageRepository: kafkaFailedMsgRepo,
		ProviderPool:                 providerPool,
		HeadTracker:                  headTracker,
		AddressMonitor:               addressMonitor,
		TransactionConfirmManager:    confirmManager,
		BalanceValidator:             balanceValidator,
		AddressLoader:                addressLoader,
		KafkaProducer:                kafkaProducer,
		KafkaRetryQueue:              kafkaRetryQueue,
	}

	return sc
}

func (sc *ServiceContext) Start(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	sc.startOnce.Do(func() {
		logx.Info("🚀 Starting chainsync components...")

		if sc.ProviderPool != nil {
			sc.ProviderPool.StartHealthCheck()
		}

		if sc.HeadTracker != nil {
			sc.HeadTracker.Start(ctx)
		}

		if sc.AddressMonitor != nil {
			go sc.AddressMonitor.StartMonitoringWithConfig(sc.Config)
		}

		if sc.TransactionConfirmManager != nil {
			go sc.TransactionConfirmManager.Start()
		}

		if sc.BalanceValidator != nil {
			go sc.BalanceValidator.StartWithConfig(sc.Config)
		}

		go func() {
			<-ctx.Done()
			sc.Stop()
		}()

		logx.Info("✅ Chainsync components started")
	})

	return nil
}

func (sc *ServiceContext) Stop() {
	sc.stopOnce.Do(func() {
		logx.Info("🛑 Stopping chainsync components...")

		if sc.AddressMonitor != nil {
			sc.AddressMonitor.StopMonitoring()
		}

		if sc.TransactionConfirmManager != nil {
			sc.TransactionConfirmManager.Stop()
		}

		if sc.BalanceValidator != nil {
			sc.BalanceValidator.Stop()
		}

		if sc.HeadTracker != nil {
			sc.HeadTracker.Stop()
		}

		if sc.ProviderPool != nil {
			sc.ProviderPool.Stop()
		}

		// 注意: KafkaRetryQueue目前由TransactionConfirmManager管理，这里不需要单独停止

		if sc.KafkaProducer != nil {
			if err := sc.KafkaProducer.Close(); err != nil {
				logx.Errorf("Error closing kafka producer: %v", err)
			}
		}

		if sc.DB != nil {
			if sqlDB, err := sc.DB.DB(); err == nil {
				if closeErr := sqlDB.Close(); closeErr != nil {
					logx.Errorf("Error closing database: %v", closeErr)
				}
			}
		}

		if sc.RedisClient != nil {
			if err := sc.RedisClient.Close(); err != nil {
				logx.Errorf("Error closing Redis: %v", err)
			}
		}

		logx.Info("✅ Chainsync components stopped")
	})
}

// autoMigrate 自动迁移数据库表
func autoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&chainModel.BlockGorm{},
		&chainModel.TransactionGorm{},
		&chainModel.TransactionLogGorm{},
		&chainModel.AddressMonitorGorm{},
		&chainModel.SyncTaskGorm{},
		&chainModel.ProviderStatusGorm{},
		&chainModel.UnconfirmedTransaction{},
		&chainModel.KafkaFailedMessage{},
	)
}

// initProviders 初始化所有服务商并添加到统一池
func initProviders(pool *provider.Pool, c *config.Config) {
	logx.Info("🚀 Initializing providers...")

	// 初始化自建节点服务商（优先使用）
	if len(c.Providers.SelfHosted) > 0 {
		logx.Info("🏠 Initializing self-hosted providers...")
		for i, selfHostedConfig := range c.Providers.SelfHosted {
			if !selfHostedConfig.Enabled || selfHostedConfig.Endpoint == "" {
				logx.Infof("⚠️  Self-hosted provider [%d] %s is disabled or no endpoint", i, selfHostedConfig.Name)
				continue
			}

			var chainType pb.BlockChainType
			providerType := ""
			switch selfHostedConfig.Type {
			case "ethereum", "eth":
				chainType = pb.BlockChainType_CHAIN_TYPE_ETHEREUM
				providerType = "ethereum"
			case "bsc", "BSC":
				chainType = pb.BlockChainType_CHAIN_TYPE_BSC
				providerType = "bsc"
			case "tron", "TRON", "trc20":
				chainType = pb.BlockChainType_CHAIN_TYPE_TRON
				providerType = "tron"
			default:
				logx.Errorf("❌ Self-hosted provider [%d] unknown chain type: %s", i, selfHostedConfig.Type)
				continue
			}

			var chainID int64
			switch chainType {
			case pb.BlockChainType_CHAIN_TYPE_ETHEREUM:
				chainID = c.Chains.Ethereum.ChainID
			case pb.BlockChainType_CHAIN_TYPE_BSC:
				chainID = c.Chains.BSC.ChainID
			case pb.BlockChainType_CHAIN_TYPE_TRON:
				chainID = c.Chains.Tron.ChainID
			default:
				chainID = 0
			}

			selfHostedProvider, err := selfhosted.NewSelfHostedProvider(&selfhosted.Config{
				Endpoint: selfHostedConfig.Endpoint,
				Name:     selfHostedConfig.Name,
				Chain:    chainType,
				ChainID:  chainID,
				APIKey:   selfHostedConfig.APIKey,
				Type:     providerType,
				Weight:   selfHostedConfig.Weight,
			})
			if err != nil {
				logx.Errorf("❌ Failed to initialize self-hosted provider %s: %v", selfHostedConfig.Name, err)
				continue
			}

			pool.AddProvider(selfHostedProvider)
			logx.Infof("✅ Self-hosted provider initialized: %s (Chain: %v)", selfHostedConfig.Name, chainType)
		}
	}

	// 初始化标准服务商（作为备用）
	providers := []struct {
		name   string
		config config.ProviderConfig
		chain  pb.BlockChainType
	}{
		{"Ethereum", c.Providers.Ethereum, pb.BlockChainType_CHAIN_TYPE_ETHEREUM},
		{"QuickNode", c.Providers.QuickNode, pb.BlockChainType_CHAIN_TYPE_ETHEREUM},
		{"Infura", c.Providers.Infura, pb.BlockChainType_CHAIN_TYPE_ETHEREUM},
		{"Alchemy", c.Providers.Alchemy, pb.BlockChainType_CHAIN_TYPE_ETHEREUM},
		{"BSC", c.Providers.BSC, pb.BlockChainType_CHAIN_TYPE_BSC},
		{"Tron", c.Providers.Tron, pb.BlockChainType_CHAIN_TYPE_TRON},
	}

	for _, info := range providers {
		if !info.config.Enabled || info.config.Endpoint == "" {
			continue
		}

		var p provider.Provider
		var err error

		switch info.chain {
		case pb.BlockChainType_CHAIN_TYPE_TRON:
			p, err = tron.NewTronProvider(info.name, info.chain, info.config.Endpoint, c.Chains.Tron.ChainID, info.config.APIKey, info.config.Weight)
		case pb.BlockChainType_CHAIN_TYPE_ETHEREUM:
			p, err = evm.NewWeb3Provider(info.name, info.chain, info.config.Endpoint, c.Chains.Ethereum.ChainID, info.config.Weight)
		case pb.BlockChainType_CHAIN_TYPE_BSC:
			p, err = evm.NewWeb3Provider(info.name, info.chain, info.config.Endpoint, c.Chains.BSC.ChainID, info.config.Weight)
		default:
			logx.Errorf("Unknown chain type: %v", info.chain)
			continue
		}

		if err != nil {
			logx.Errorf("❌ Failed to initialize %s provider: %v", info.name, err)
			continue
		}

		pool.AddProvider(p)
		logx.Infof("✅ %s provider initialized: %s", info.name, info.config.Endpoint)
	}

	logx.Info("🚀 All providers initialized (health check will start on ServiceContext.Start)")
}
