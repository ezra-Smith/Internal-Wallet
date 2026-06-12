package svc

import (
	"context"
	"time"

	"internalwallet/common/cache"
	commonDB "internalwallet/common/db"
	"internalwallet/common/utils"
	"internalwallet/services/signer/rpc/internal/config"
	"internalwallet/services/signer/rpc/internal/models"
	"internalwallet/services/signer/rpc/internal/repository"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type ServiceContext struct {
	Config      config.Config
	DB          *gorm.DB
	RedisClient *redis.Client
	SignerCache *cache.RedisCache

	// Repository层
	MasterSeedRepo           repository.MasterSeedRepository
	SignatureLogRepo         repository.SignatureLogRepository
	AddressGenerationLogRepo repository.AddressGenerationLogRepository
	MnemonicBackupRepo       repository.MnemonicBackupRepository
	WalletMasterMnemonicRepo repository.WalletMasterMnemonicRepository
	CompanyWalletRepo        repository.CompanyWalletRepository

	// Wallet runtime (in-memory)
	WalletRuntime *WalletRuntime
}

func NewServiceContext(c config.Config) *ServiceContext {
	// 初始化雪花ID生成器（必须在服务启动时初始化）
	if err := utils.Init(c.NodeID); err != nil {
		logx.Severef("Failed to initialize snowflake ID generator: %v", err)
	}
	logx.Infof("Snowflake ID generator initialized with NodeID: %d", c.NodeID)

	// 初始化 GORM 数据库连接（使用common/db）
	var db *gorm.DB
	var err error
	if c.MySQL.Database != "" {
		// Cold start can race with CNI/sidecar readiness; retry a bit to avoid permanent nil repos.
		const maxAttempts = 12
		backoff := 300 * time.Millisecond
		for attempt := 1; attempt <= maxAttempts; attempt++ {
			db, err = commonDB.InitGorm(c.MySQL)
			if err == nil && db != nil {
				break
			}
			logx.Errorf("Failed to initialize GORM (attempt %d/%d): %v", attempt, maxAttempts, err)
			time.Sleep(backoff)
			if backoff < 3*time.Second {
				backoff *= 2
				if backoff > 3*time.Second {
					backoff = 3 * time.Second
				}
			}
		}

		if err == nil && db != nil {
			logx.Info("✓ GORM initialized successfully for signer service")

			// 自动迁移数据库表（仅开发/测试环境）
			if c.AutoMigrate && (c.Mode == "dev" || c.Mode == "test") {
				if err := db.AutoMigrate(
					&models.MasterSeed{},
					&models.AddressGenerationLog{},
					&models.SignatureLog{},
					&models.PubkeyExportLog{},
					&models.HDWalletConfig{},
					&models.MnemonicBackup{},
					&models.CompanyWallet{},
					&models.WalletMasterMnemonic{},
				); err != nil {
					logx.Errorf("Failed to auto migrate database tables: %v", err)
				} else {
					logx.Info("✓ Database tables auto-migrated successfully (dev/test mode)")
				}
			} else if c.AutoMigrate && c.Mode == "pro" {
				logx.Error("AutoMigrate is disabled in production mode for safety")
			}
		} else {
			logx.Errorf("Failed to initialize GORM after retries; signer DB features will be unavailable")
		}
	} else {
		logx.Errorf("MySQL config not found in signer service")
	}

	// 创建 Redis 客户端
	redisClient := redis.NewClient(&redis.Options{
		Addr:     c.CacheRedis[0].Host,
		Password: c.CacheRedis[0].Pass,
		DB:       0,
	})

	// 测试 Redis 连接
	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		logx.Errorf("Failed to connect to Redis: %v", err)
	} else {
		logx.Info("✓ Connected to Redis for signer service")
	}

	// 创建缓存实例
	signerCache := cache.NewRedisCache(redisClient, "signer")

	// TODO: 初始化密钥存储
	// 生产环境应使用HSM或安全的密钥管理系统

	// 验证加密密码配置
	if c.Security.EncryptionPassword == "" || c.Security.EncryptionPassword == "CHANGE_ME_IN_PRODUCTION_USE_ENV_VAR" {
		logx.Errorf("WARNING: Encryption password not properly configured! Please set it via environment variable.")
	}

	// 初始化Repository层
	var masterSeedRepo repository.MasterSeedRepository
	var signatureLogRepo repository.SignatureLogRepository
	var addressGenerationLogRepo repository.AddressGenerationLogRepository
	var mnemonicBackupRepo repository.MnemonicBackupRepository
	var walletMasterMnemonicRepo repository.WalletMasterMnemonicRepository
	var companyWalletRepo repository.CompanyWalletRepository
	if db != nil {
		masterSeedRepo = repository.NewMasterSeedRepository(db)
		signatureLogRepo = repository.NewSignatureLogRepository(db)
		addressGenerationLogRepo = repository.NewAddressGenerationLogRepository(db)
		mnemonicBackupRepo = repository.NewMnemonicBackupRepository(db)
		walletMasterMnemonicRepo = repository.NewWalletMasterMnemonicRepository(db)
		companyWalletRepo = repository.NewCompanyWalletRepository(db)
	} else {
		logx.Errorf("Signer DB is not ready; repositories will be nil")
	}

	logx.Info("Initialized all repositories for signer service")

	return &ServiceContext{
		Config:                   c,
		DB:                       db,
		RedisClient:              redisClient,
		SignerCache:              signerCache,
		MasterSeedRepo:           masterSeedRepo,
		SignatureLogRepo:         signatureLogRepo,
		AddressGenerationLogRepo: addressGenerationLogRepo,
		MnemonicBackupRepo:       mnemonicBackupRepo,
		WalletMasterMnemonicRepo: walletMasterMnemonicRepo,
		CompanyWalletRepo:        companyWalletRepo,
		WalletRuntime:            NewWalletRuntime(),
	}
}
