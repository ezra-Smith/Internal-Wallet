package svc

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	commonDB "internalwallet/common/db"
	"internalwallet/common/utils"
	"internalwallet/pkg/chainnode"
	"internalwallet/services/consolidation/rpc/internal/config"
	"internalwallet/services/consolidation/rpc/internal/models"
	"internalwallet/services/consolidation/rpc/internal/repository"
	"internalwallet/services/signer/rpc/signerservice"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/zrpc"
	"gorm.io/gorm"
)

type ServiceContext struct {
	Config config.Config

	DB          *gorm.DB
	RedisClient *redis.Client

	Chain     chainnode.Client
	SignerRpc signerservice.SignerService

	// repositories
	AddressRepo           repository.AddressRepository
	ConsolidationTaskRepo repository.ConsolidationTaskRepository
	ConsolidationLogRepo  repository.ConsolidationLogRepository
	EnergyRentalRepo      repository.EnergyRentalRecordRepository
	TopUpRecordRepo       repository.TopUpRecordRepository
	CurrencyChainRepo     repository.CurrencyChainSettingsRepository

	InstanceID string

	runtimeMu  sync.Mutex
	runtimeCfg atomic.Value // stores *RuntimeConfig
}

func NewServiceContext(c config.Config) *ServiceContext {
	// Instance ID for claiming/locking (stable per process start).
	instanceID := fmt.Sprintf("%s-%d", hostnameOrUnknown(), time.Now().UnixNano())

	// Init snowflake generator.
	if err := utils.Init(c.NodeID); err != nil {
		logx.Severef("Failed to initialize snowflake ID generator: %v", err)
	}
	logx.Infof("Snowflake ID generator initialized with NodeID: %d", c.NodeID)

	// Init DB.
	var db *gorm.DB
	var err error
	if c.MySQL.Database != "" {
		db, err = commonDB.InitGorm(c.MySQL)
		if err != nil {
			logx.Severef("Failed to initialize GORM: %v", err)
		}
		logx.Info("✓ GORM initialized successfully for consolidation service")

		// Auto-migrate in dev/test only (optional).
		if c.AutoMigrate && (c.Mode == "dev" || c.Mode == "test") {
			if err := db.AutoMigrate(
				&models.ConsolidationTask{},
				&models.ConsolidationLog{},
				&models.EnergyRentalRecord{},
				&models.ConsolidationTopUpRecord{},
			); err != nil {
				logx.Errorf("Failed to auto migrate database tables: %v", err)
			} else {
				logx.Info("✓ Database tables auto-migrated successfully (dev/test mode)")
			}
		} else if c.AutoMigrate && c.Mode == "pro" {
			logx.Error("AutoMigrate is disabled in production mode for safety")
		}
	} else {
		logx.Severef("MySQL config not found in consolidation service")
	}

	// Optional Redis client.
	var redisClient *redis.Client
	if len(c.CacheRedis) > 0 && c.CacheRedis[0].Host != "" {
		redisClient = redis.NewClient(&redis.Options{
			Addr:         c.CacheRedis[0].Host,
			Password:     c.CacheRedis[0].Pass,
			DB:           0,
			PoolSize:     50,
			MinIdleConns: 10,
			MaxRetries:   3,
			DialTimeout:  5 * time.Second,
			ReadTimeout:  3 * time.Second,
			WriteTimeout: 3 * time.Second,
		})
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := redisClient.Ping(ctx).Err(); err != nil {
			logx.Errorf("Failed to connect to Redis (%s): %v (will continue without Redis)", c.CacheRedis[0].Host, err)
			redisClient = nil
		} else {
			logx.Infof("✓ Connected to Redis (%s) for consolidation service", c.CacheRedis[0].Host)
		}
	}

	// Direct chain node clients.
	var chainClient chainnode.Client
	if len(c.Chains) > 0 {
		if !c.Consolidation.Enabled {
			logx.Info("Consolidation.Enabled=false; skipping chain node client initialization")
		} else {
			cli, err := chainnode.New(c.Chains)
			if err != nil {
				logx.Severef("Failed to initialize chain node clients: %v", err)
			} else {
				chainClient = cli
				logx.Info("✓ Chain node clients initialized successfully")
			}
		}
	} else {
		if c.Consolidation.Enabled {
			logx.Severef("Chains config not found in consolidation service")
		} else {
			logx.Info("Chains config not found, consolidation execution will be disabled")
		}
	}

	// SignerRpc client.
	var signerClient signerservice.SignerService
	if len(c.SignerRpc.Etcd.Hosts) > 0 || len(c.SignerRpc.Endpoints) > 0 {
		signerConn, err := zrpc.NewClient(c.SignerRpc)
		if err != nil {
			logx.Errorf("Failed to connect to SignerRpc: %v (service may not be started yet)", err)
		} else {
			signerClient = signerservice.NewSignerService(signerConn)
			logx.Info("✓ SignerRpc client initialized successfully")
		}
	} else {
		logx.Info("SignerRpc config not found, transaction signing will be disabled")
	}

	// repositories
	addressRepo := repository.NewAddressRepository(db)
	taskRepo := repository.NewConsolidationTaskRepository(db)
	logRepo := repository.NewConsolidationLogRepository(db)
	energyRepo := repository.NewEnergyRentalRecordRepository(db)
	topupRepo := repository.NewTopUpRecordRepository(db)
	currencyChainRepo := repository.NewCurrencyChainSettingsRepository(db)

	svcCtx := &ServiceContext{
		Config:      c,
		DB:          db,
		RedisClient: redisClient,
		Chain:       chainClient,
		SignerRpc:   signerClient,

		AddressRepo:           addressRepo,
		ConsolidationTaskRepo: taskRepo,
		ConsolidationLogRepo:  logRepo,
		EnergyRentalRepo:      energyRepo,
		TopUpRecordRepo:       topupRepo,
		CurrencyChainRepo:     currencyChainRepo,

		InstanceID: instanceID,
	}

	// Initialize runtime snapshot to avoid atomic.Value panics on Load().
	svcCtx.runtimeCfg.Store(NewEmptyRuntimeConfig())
	return svcCtx
}

func hostnameOrUnknown() string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		return "unknown-host"
	}
	return h
}
