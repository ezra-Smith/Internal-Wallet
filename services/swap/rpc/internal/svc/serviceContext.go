package svc

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/zrpc"
	"gorm.io/gorm"

	"internalwallet/common/cache"
	commonDB "internalwallet/common/db"
	"internalwallet/common/utils"
	"internalwallet/proto/pb"
	"internalwallet/services/swap/rpc/internal/config"
	"internalwallet/services/swap/rpc/internal/model"
	"internalwallet/services/swap/rpc/internal/provider"
	_ "internalwallet/services/swap/rpc/internal/provider/okx"
	_ "internalwallet/services/swap/rpc/internal/provider/oneinch"
	"internalwallet/services/swap/rpc/internal/repository"
)

type ServiceContext struct {
	Config config.Config

	DB          *gorm.DB
	RedisClient *redis.Client

	TokenCache *cache.RedisCache

	EVM *EVMClientManager

	ApiKeyRepo repository.SwapApiKeyRepository

	Provider provider.SwapProvider

	SwapProviderRepo repository.SwapProviderRepository

	TxRepo repository.SwapTransactionRepository

	SwapConfigRepo         repository.SwapConfigRepository
	SwapProjectEnabledRepo repository.SwapProjectEnabledRepository

	SignerRpc pb.SignerServiceClient
}

func NewServiceContext(c config.Config) *ServiceContext {
	// 初始化雪花ID生成器（必须）
	if err := utils.Init(c.NodeID); err != nil {
		logx.Severef("Failed to initialize snowflake ID generator: %v", err)
	}
	logx.Infof("Snowflake ID generator initialized with NodeID: %d", c.NodeID)

	// 初始化 Redis（可选但建议配置）
	var redisClient *redis.Client
	if len(c.CacheRedis) > 0 && c.CacheRedis[0].Host != "" {
		redisClient = redis.NewClient(&redis.Options{
			Addr:         c.CacheRedis[0].Host,
			Password:     c.CacheRedis[0].Pass,
			DB:           0,
			DialTimeout:  5 * time.Second,
			ReadTimeout:  3 * time.Second,
			WriteTimeout: 3 * time.Second,
			PoolSize:     20,
			MinIdleConns: 5,
		})
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := redisClient.Ping(ctx).Err(); err != nil {
			logx.Errorf("Failed to connect to Redis: %v", err)
		} else {
			logx.Info("✓ Connected to Redis for swap service")
		}
	}

	// 初始化 GORM（可选但推荐配置）
	var gormDB *gorm.DB
	if c.MySQL.Database != "" {
		db, err := commonDB.InitGorm(c.MySQL)
		if err != nil {
			logx.Errorf("Failed to initialize GORM: %v", err)
		} else {
			gormDB = db
			logx.Info("✓ GORM initialized successfully")

			if c.AutoMigrate {
				if err := gormDB.AutoMigrate(
					&model.SwapSvcApiKeyModel{},
					&model.SwapSvcTransactionModel{},
					&model.SwapSvcFeeConfigModel{},

					&model.SwapProviderModel{},
					&model.SwapConfigModel{},
					&model.SwapProjectEnabledModel{},
				); err != nil {
					logx.Errorf("AutoMigrate failed: %v", err)
				} else {
					logx.Info("✓ AutoMigrate completed for swap models")
				}
			}
		}
	}

	// 初始化缓存
	var tokenCache *cache.RedisCache
	if redisClient != nil {
		tokenCache = cache.NewRedisCache(redisClient, "swap_tokens")
	}

	evmMgr := NewEVMClientManager(c.Swap.Chains)

	var apiKeyRepo repository.SwapApiKeyRepository
	var swapProviderRepo repository.SwapProviderRepository
	var txRepo repository.SwapTransactionRepository
	var swapConfigRepo repository.SwapConfigRepository
	var swapProjectEnabledRepo repository.SwapProjectEnabledRepository

	if gormDB != nil {
		apiKeyRepo = repository.NewSwapApiKeyRepository(gormDB)
		swapProviderRepo = repository.NewSwapProviderRepository(gormDB)
		txRepo = repository.NewSwapTransactionRepository(gormDB)
		swapConfigRepo = repository.NewSwapConfigRepository(gormDB)
		swapProjectEnabledRepo = repository.NewSwapProjectEnabledRepository(gormDB)
	}

	var swapProvider provider.SwapProvider
	logx.Infof("=== [SWAP-SVC] Initializing Swap Provider ===")
	logx.Infof("=== [SWAP-SVC] Provider Type: %s ===", c.Swap.Provider.Type)
	logx.Infof("=== [SWAP-SVC] Number of chains configured: %d ===", len(c.Swap.Chains))

	if p, err := provider.NewProvider(c.Swap.Provider, c.Swap.Chains); err != nil {
		logx.Errorf("=== [SWAP-SVC] Failed to initialize swap provider: %v ===", err)
	} else {
		swapProvider = p
		logx.Info("✓ Swap provider initialized")
	}

	// 初始化 Signer RPC 客户端（用于交易签名）
	var signerRpc pb.SignerServiceClient
	if len(c.SignerRpc.Etcd.Hosts) > 0 {
		signerConn := zrpc.MustNewClient(c.SignerRpc)
		signerRpc = pb.NewSignerServiceClient(signerConn.Conn())
		logx.Info("✓ Connected to Signer RPC service")
	}

	return &ServiceContext{
		Config:                 c,
		DB:                     gormDB,
		RedisClient:            redisClient,
		TokenCache:             tokenCache,
		EVM:                    evmMgr,
		ApiKeyRepo:             apiKeyRepo,
		Provider:               swapProvider,
		SwapProviderRepo:       swapProviderRepo,
		TxRepo:                 txRepo,
		SwapConfigRepo:         swapConfigRepo,
		SwapProjectEnabledRepo: swapProjectEnabledRepo,
		SignerRpc:              signerRpc,
	}
}
