package svc

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/zrpc"
	"gorm.io/gorm"

	commonDB "internalwallet/common/db"
	"internalwallet/common/mq"
	"internalwallet/common/utils"
	"internalwallet/pkg/notify"
	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/config"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/repository"
	"internalwallet/services/chainrpc/rpc/chainrpc"
)

type ServiceContext struct {
	Config config.Config

	DB          *gorm.DB
	RedisClient *redis.Client

	AdminUserRepo       repository.AdminUserRepository
	AdminLoginLogRepo   repository.AdminLoginLogRepository
	AdminAuditLogRepo   repository.AdminAuditLogRepository
	SystemConfigRepo    repository.AdminSystemConfigRepository
	PasswordHistoryRepo repository.AdminPasswordHistoryRepository
	BackupCodeRepo      repository.AdminBackupCodeRepository

	// Web2 Users
	UserRepo                  repository.UserRepository
	UserAdminNoteRepo         repository.UserAdminNoteRepository
	UserWhitelistSettingsRepo repository.UserWhitelistSettingsRepository
	User2FAHistoryRepo        repository.User2FAHistoryRepository

	// RBAC v2
	AdminMenuRepo           repository.AdminMenuRepository
	AdminPermissionRepo     repository.AdminPermissionRepository
	AdminRoleRepo           repository.AdminRoleRepository
	AdminRoleMenuRepo       repository.AdminRoleMenuRepository
	AdminRolePermissionRepo repository.AdminRolePermissionRepository
	AdminUserRoleRepo       repository.AdminUserRoleRepository
	AdminRBACRepo           repository.AdminRBACRepository

	SessionManager     *SessionManager
	TwoFAManager       *TwoFAManager
	MFAFlow            *MFAFlowManager
	TokenBlacklist     *TokenBlacklistManager
	TokenRefresh       *TokenRefreshManager
	IdempotencyManager *IdempotencyManager

	// Wallet Assets (admin-managed)
	WalletDepositAddressRepo        repository.WalletDepositAddressRepository
	WalletDepositAddressBalanceRepo repository.WalletDepositAddressBalanceRepository
	WalletDepositRepo               repository.WalletDepositRepository

	// Currency Management (business-facing assets/chains)
	ChainRepo                           repository.ChainRepository
	CurrencySettingsRepo                repository.CurrencySettingsRepository
	CurrencyChainSettingsRepo           repository.CurrencyChainSettingsRepository
	CurrencyWithdrawFeeRuleRepo         repository.CurrencyWithdrawFeeRuleRepository
	CurrencyGlobalWithdrawFeeRuleRepo   repository.CurrencyGlobalWithdrawFeeRuleRepository
	CurrencyWithdrawAuditRuleRepo       repository.CurrencyWithdrawAuditRuleRepository
	CurrencyGlobalWithdrawAuditRuleRepo repository.CurrencyGlobalWithdrawAuditRuleRepository
	CurrencyGlobalTransferAuditRuleRepo repository.CurrencyGlobalTransferAuditRuleRepository
	CurrencyWithdrawOrderRepo           repository.CurrencyWithdrawOrderRepository
	CurrencyWithdrawOrderEventRepo      repository.CurrencyWithdrawOrderEventRepository
	CurrencyWithdrawPayoutTaskRepo      repository.CurrencyWithdrawPayoutTaskRepository
	CurrencyTransferOrderRepo           repository.CurrencyTransferOrderRepository
	UserTransactionRecordRepo           repository.UserTransactionRecordRepository

	// Transfer (batch payout)
	TransferBatchRepo     repository.TransferBatchRepository
	TransferBatchItemRepo repository.TransferBatchItemRepository

	// Web3 Users (device-based)
	Web3UserRepo          repository.Web3UserRepository
	Web3UserAddressRepo   repository.Web3UserAddressRepository
	Web3TransactionRepo   repository.Web3TransactionRepository
	Web3BalanceChangeRepo repository.Web3BalanceChangeRepository

	// Blacklist (on-chain sensitive addresses)
	BlacklistAddressRepo repository.BlacklistAddressRepository

	// Vault (funds management)
	VaultNetworkRepo            repository.VaultNetworkRepository
	VaultBalanceRepo            repository.VaultBalanceRepository
	VaultThresholdRepo          repository.VaultThresholdRepository
	VaultAdjustmentRepo         repository.VaultAdjustmentRepository
	VaultAdjustmentApprovalRepo repository.VaultAdjustmentApprovalRepository
	VaultSyncTaskRepo           repository.VaultSyncTaskRepository
	VaultAddressRepo            repository.VaultAddressRepository
	VaultAddressBalanceRepo     repository.VaultAddressBalanceRepository

	ChainSyncClient  pb.ChainSyncClient
	ChainRpc         chainrpc.ChainRPC
	SignerRpc        pb.SignerServiceClient
	AccountingRpc    pb.AccountingClient
	SwapRpc          pb.SwapClient
	ConsolidationRpc pb.ConsolidationClient

	hotWalletStateCache *hotWalletStateCache

	// Alert Config Repository (moved from Alert service to Admin)
	AlertConfigRepo repository.AlertConfigRepository

	AddressMonitorEventProducer mq.KafkaProducer
}

func NewServiceContext(c config.Config) *ServiceContext {
	// 初始化雪花ID生成器（必须）
	if err := utils.Init(c.NodeID); err != nil {
		logx.Severef("Failed to initialize snowflake ID generator: %v", err)
	}
	logx.Infof("Snowflake ID generator initialized with NodeID: %d", c.NodeID)

	// 默认 SMTP 配置（环境变量优先覆盖）
	notify.SetDefaultEmailConfig(notify.EmailConfig{
		SMTPHost:           c.Email.SMTPHost,
		SMTPPort:           c.Email.SMTPPort,
		FromAddress:        c.Email.FromAddress,
		FromPassword:       c.Email.FromPassword,
		FromName:           c.Email.FromName,
		InsecureSkipVerify: c.Email.InsecureSkipVerify,

		// 邮件模板配置
		FreezeAccountURL: c.Email.FreezeAccountURL,
		SupportEmail:     c.Email.SupportEmail,
		CompanyName:      c.Email.CompanyName,

		// S3 图片资源 URL
		LogoURL:           c.Email.LogoURL,
		SocialXURL:        c.Email.SocialXURL,
		SocialTelegramURL: c.Email.SocialTelegramURL,
		SocialTikTokURL:   c.Email.SocialTikTokURL,
		SocialLinkedInURL: c.Email.SocialLinkedInURL,
		SocialFacebookURL: c.Email.SocialFacebookURL,
		SocialRedditURL:   c.Email.SocialRedditURL,
		AppGooglePlayURL:  c.Email.AppGooglePlayURL,
		AppAppStoreURL:    c.Email.AppAppStoreURL,
	})

	// 初始化 Redis（可选但强烈建议配置）
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
			logx.Info("✓ Connected to Redis for admin service")
		}
	}

	// 初始化 GORM
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
					&model.AdminUserModel{},
					&model.AdminLoginLogModel{},
					&model.AdminAuditLogModel{},
					&model.AdminSystemConfigModel{},
					&model.AdminPasswordHistoryModel{},
					&model.AdminBackupCodeModel{},
					&model.UserAdminNoteModel{},
					&model.UserWhitelistSettingsModel{},
					&model.UserWithdrawAuditWhitelistRuleModel{},
					&model.User2FAHistoryModel{},
					&model.ChainModel{},
					&model.CurrencySettingsModel{},
					&model.CurrencyChainSettingsModel{},
					&model.CurrencyWithdrawFeeRuleModel{},
					&model.CurrencyGlobalWithdrawFeeRuleModel{},
					&model.CurrencyWithdrawAuditRuleModel{},
					&model.CurrencyGlobalWithdrawAuditRuleModel{},
					&model.CurrencyWithdrawOrderModel{},
				); err != nil {
					logx.Errorf("AutoMigrate failed: %v", err)
				} else {
					logx.Info("✓ AutoMigrate completed for admin models")
				}
			}
		}
	}

	// 初始化 Repository
	var adminUserRepo repository.AdminUserRepository
	var loginLogRepo repository.AdminLoginLogRepository
	var auditLogRepo repository.AdminAuditLogRepository
	var systemConfigRepo repository.AdminSystemConfigRepository
	var passwordHistoryRepo repository.AdminPasswordHistoryRepository
	var backupCodeRepo repository.AdminBackupCodeRepository
	var adminMenuRepo repository.AdminMenuRepository
	var adminPermissionRepo repository.AdminPermissionRepository
	var adminRoleRepo repository.AdminRoleRepository
	var adminRoleMenuRepo repository.AdminRoleMenuRepository
	var adminRolePermissionRepo repository.AdminRolePermissionRepository
	var adminUserRoleRepo repository.AdminUserRoleRepository
	var adminRBACRepo repository.AdminRBACRepository
	var userRepo repository.UserRepository
	var userAdminNoteRepo repository.UserAdminNoteRepository
	var userWhitelistSettingsRepo repository.UserWhitelistSettingsRepository
	var user2FAHistoryRepo repository.User2FAHistoryRepository
	var walletDepositAddressRepo repository.WalletDepositAddressRepository
	var walletDepositAddressBalanceRepo repository.WalletDepositAddressBalanceRepository
	var walletDepositRepo repository.WalletDepositRepository
	var chainRepo repository.ChainRepository
	var currencySettingsRepo repository.CurrencySettingsRepository
	var currencyChainSettingsRepo repository.CurrencyChainSettingsRepository
	var currencyWithdrawFeeRuleRepo repository.CurrencyWithdrawFeeRuleRepository
	var currencyGlobalWithdrawFeeRuleRepo repository.CurrencyGlobalWithdrawFeeRuleRepository
	var currencyWithdrawAuditRuleRepo repository.CurrencyWithdrawAuditRuleRepository
	var currencyGlobalWithdrawAuditRuleRepo repository.CurrencyGlobalWithdrawAuditRuleRepository
	var currencyGlobalTransferAuditRuleRepo repository.CurrencyGlobalTransferAuditRuleRepository
	var currencyWithdrawOrderRepo repository.CurrencyWithdrawOrderRepository
	var currencyWithdrawOrderEventRepo repository.CurrencyWithdrawOrderEventRepository
	var currencyWithdrawPayoutTaskRepo repository.CurrencyWithdrawPayoutTaskRepository
	var currencyTransferOrderRepo repository.CurrencyTransferOrderRepository
	var userTransactionRecordRepo repository.UserTransactionRecordRepository
	var transferBatchRepo repository.TransferBatchRepository
	var transferBatchItemRepo repository.TransferBatchItemRepository
	var web3UserRepo repository.Web3UserRepository
	var web3UserAddressRepo repository.Web3UserAddressRepository
	var web3TransactionRepo repository.Web3TransactionRepository
	var web3BalanceChangeRepo repository.Web3BalanceChangeRepository
	var blacklistAddressRepo repository.BlacklistAddressRepository
	var vaultNetworkRepo repository.VaultNetworkRepository
	var vaultBalanceRepo repository.VaultBalanceRepository
	var vaultThresholdRepo repository.VaultThresholdRepository
	var vaultAdjustmentRepo repository.VaultAdjustmentRepository
	var vaultAdjustmentApprovalRepo repository.VaultAdjustmentApprovalRepository
	var vaultSyncTaskRepo repository.VaultSyncTaskRepository
	var vaultAddressRepo repository.VaultAddressRepository
	var vaultAddressBalanceRepo repository.VaultAddressBalanceRepository
	if gormDB != nil {
		adminUserRepo = repository.NewAdminUserRepository(gormDB)
		loginLogRepo = repository.NewAdminLoginLogRepository(gormDB)
		auditLogRepo = repository.NewAdminAuditLogRepository(gormDB)
		systemConfigRepo = repository.NewAdminSystemConfigRepository(gormDB)
		passwordHistoryRepo = repository.NewAdminPasswordHistoryRepository(gormDB)
		backupCodeRepo = repository.NewAdminBackupCodeRepository(gormDB)
		adminMenuRepo = repository.NewAdminMenuRepository(gormDB)
		adminPermissionRepo = repository.NewAdminPermissionRepository(gormDB)
		adminRoleRepo = repository.NewAdminRoleRepository(gormDB)
		adminRoleMenuRepo = repository.NewAdminRoleMenuRepository(gormDB)
		adminRolePermissionRepo = repository.NewAdminRolePermissionRepository(gormDB)
		adminUserRoleRepo = repository.NewAdminUserRoleRepository(gormDB)
		adminRBACRepo = repository.NewAdminRBACRepository(gormDB)

		userRepo = repository.NewUserRepository(gormDB)
		userAdminNoteRepo = repository.NewUserAdminNoteRepository(gormDB)
		userWhitelistSettingsRepo = repository.NewUserWhitelistSettingsRepository(gormDB)
		user2FAHistoryRepo = repository.NewUser2FAHistoryRepository(gormDB)

		walletDepositAddressRepo = repository.NewWalletDepositAddressRepository(gormDB)
		walletDepositAddressBalanceRepo = repository.NewWalletDepositAddressBalanceRepository(gormDB)
		walletDepositRepo = repository.NewWalletDepositRepository(gormDB)

		chainRepo = repository.NewChainRepository(gormDB)
		currencySettingsRepo = repository.NewCurrencySettingsRepository(gormDB)
		currencyChainSettingsRepo = repository.NewCurrencyChainSettingsRepository(gormDB)
		currencyWithdrawFeeRuleRepo = repository.NewCurrencyWithdrawFeeRuleRepository(gormDB)
		currencyGlobalWithdrawFeeRuleRepo = repository.NewCurrencyGlobalWithdrawFeeRuleRepository(gormDB)
		currencyWithdrawAuditRuleRepo = repository.NewCurrencyWithdrawAuditRuleRepository(gormDB)
		currencyGlobalWithdrawAuditRuleRepo = repository.NewCurrencyGlobalWithdrawAuditRuleRepository(gormDB)
		currencyGlobalTransferAuditRuleRepo = repository.NewCurrencyGlobalTransferAuditRuleRepository(gormDB)
		currencyWithdrawOrderRepo = repository.NewCurrencyWithdrawOrderRepository(gormDB)
		currencyWithdrawOrderEventRepo = repository.NewCurrencyWithdrawOrderEventRepository(gormDB)
		currencyWithdrawPayoutTaskRepo = repository.NewCurrencyWithdrawPayoutTaskRepository(gormDB)
		currencyTransferOrderRepo = repository.NewCurrencyTransferOrderRepository(gormDB)
		userTransactionRecordRepo = repository.NewUserTransactionRecordRepository(gormDB)

		transferBatchRepo = repository.NewTransferBatchRepository(gormDB)
		transferBatchItemRepo = repository.NewTransferBatchItemRepository(gormDB)

		web3UserRepo = repository.NewWeb3UserRepository(gormDB)
		web3UserAddressRepo = repository.NewWeb3UserAddressRepository(gormDB)
		web3TransactionRepo = repository.NewWeb3TransactionRepository(gormDB)
		web3BalanceChangeRepo = repository.NewWeb3BalanceChangeRepository(gormDB)

		blacklistAddressRepo = repository.NewBlacklistAddressRepository(gormDB)

		vaultNetworkRepo = repository.NewVaultNetworkRepository(gormDB)
		vaultBalanceRepo = repository.NewVaultBalanceRepository(gormDB)
		vaultThresholdRepo = repository.NewVaultThresholdRepository(gormDB)
		vaultAdjustmentRepo = repository.NewVaultAdjustmentRepository(gormDB)
		vaultAdjustmentApprovalRepo = repository.NewVaultAdjustmentApprovalRepository(gormDB)
		vaultSyncTaskRepo = repository.NewVaultSyncTaskRepository(gormDB)
		vaultAddressRepo = repository.NewVaultAddressRepository(gormDB)
		vaultAddressBalanceRepo = repository.NewVaultAddressBalanceRepository(gormDB)
	}

	// Optional ChainSync client (for Vault manual sync).
	var chainSyncClient pb.ChainSyncClient
	if len(c.ChainSyncRpc.Etcd.Hosts) > 0 || len(c.ChainSyncRpc.Endpoints) > 0 {
		conn, err := zrpc.NewClient(c.ChainSyncRpc)
		if err != nil {
			logx.Errorf("Failed to connect to ChainSyncRpc: %v (service may not be started yet)", err)
		} else {
			chainSyncClient = pb.NewChainSyncClient(conn.Conn())
			logx.Info("✓ Connected to ChainSync service (admin)")
		}
	}

	// Optional ChainRpc client (for withdrawals payout).
	var chainRpcClient chainrpc.ChainRPC
	if len(c.ChainRpc.Etcd.Hosts) > 0 || len(c.ChainRpc.Endpoints) > 0 {
		conn, err := zrpc.NewClient(c.ChainRpc)
		if err != nil {
			logx.Errorf("Failed to connect to ChainRpc: %v (service may not be started yet)", err)
		} else {
			chainRpcClient = chainrpc.NewChainRPC(conn)
			logx.Info("✓ Connected to ChainRpc service (admin)")
		}
	}

	// Optional SignerRpc client (for address generation).
	var signerRpcClient pb.SignerServiceClient
	if len(c.SignerRpc.Etcd.Hosts) > 0 || len(c.SignerRpc.Endpoints) > 0 {
		conn, err := zrpc.NewClient(c.SignerRpc)
		if err != nil {
			logx.Errorf("Failed to connect to SignerRpc: %v (service may not be started yet)", err)
		} else {
			signerRpcClient = pb.NewSignerServiceClient(conn.Conn())
			logx.Info("✓ Connected to Signer service (admin)")
		}
	}

	// Optional AccountingRpc client (for ledger/balance writes).
	var accountingRpcClient pb.AccountingClient
	if len(c.AccountingRpc.Etcd.Hosts) > 0 || len(c.AccountingRpc.Endpoints) > 0 {
		conn, err := zrpc.NewClient(c.AccountingRpc)
		if err != nil {
			logx.Errorf("Failed to connect to AccountingRpc: %v (service may not be started yet)", err)
		} else {
			accountingRpcClient = pb.NewAccountingClient(conn.Conn())
			logx.Info("✓ Connected to Accounting service (admin)")
		}
	}

	// Optional SwapRpc client (for swap configuration management).
	var swapRpcClient pb.SwapClient
	if len(c.SwapRpc.Etcd.Hosts) > 0 || len(c.SwapRpc.Endpoints) > 0 {
		conn, err := zrpc.NewClient(c.SwapRpc, zrpc.WithUnaryClientInterceptor(swapClientInterceptor(c.SwapAuth.AdminToken)))
		if err != nil {
			logx.Errorf("Failed to connect to SwapRpc: %v (service may not be started yet)", err)
		} else {
			swapRpcClient = pb.NewSwapClient(conn.Conn())
			logx.Info("✓ Connected to Swap service (admin) with authentication interceptor")
		}
	}

	// Optional ConsolidationRpc client (for consolidation workflow management).
	var consolidationRpcClient pb.ConsolidationClient
	if len(c.ConsolidationRpc.Etcd.Hosts) > 0 || len(c.ConsolidationRpc.Endpoints) > 0 {
		conn, err := zrpc.NewClient(c.ConsolidationRpc)
		if err != nil {
			logx.Errorf("Failed to connect to ConsolidationRpc: %v (service may not be started yet)", err)
		} else {
			consolidationRpcClient = pb.NewConsolidationClient(conn.Conn())
			logx.Info("✓ Connected to Consolidation service (admin)")
		}
	}

	// 初始化 Alert Config Repository
	var alertConfigRepo repository.AlertConfigRepository
	if gormDB != nil {
		alertConfigRepo = repository.NewAlertConfigRepository(gormDB)
		logx.Info("✓ Alert Config Repository initialized")
	}

	// Managers（依赖 Redis）
	sessionManager := NewSessionManager(redisClient)
	twoFAManager := NewTwoFAManager(redisClient, time.Duration(c.Security.TwoFATempTokenTTLSeconds)*time.Second)
	mfaFlow := NewMFAFlowManager(redisClient)
	tokenBlacklist := NewTokenBlacklistManager(redisClient)
	tokenRefresh := NewTokenRefreshManager(redisClient)
	idempotencyManager := NewIdempotencyManager(redisClient)

	var addressEventProducer mq.KafkaProducer
	if len(c.Kafka.Brokers) > 0 {
		p, err := mq.NewSaramaProducer(mq.KafkaProducerConfig{
			Brokers:     c.Kafka.Brokers,
			Username:    c.Kafka.Username,
			Password:    c.Kafka.Password,
			Security:    c.Kafka.Security,
			SASLMech:    c.Kafka.SASLMech,
			ClientID:    c.Kafka.ClientID,
			Compression: c.Kafka.Compression,
			MaxRetries:  c.Kafka.MaxRetries,
			Timeout:     time.Duration(c.Kafka.Timeout) * time.Second,
			UseAsync:    c.Kafka.UseAsync,
		})
		if err != nil {
			logx.Errorf("Failed to init Kafka producer (admin address monitor events): %v", err)
		} else {
			addressEventProducer = p
			logx.Infof("✓ Kafka producer initialized (admin address monitor events): brokers=%v topic=%s", c.Kafka.Brokers, strings.TrimSpace(c.Kafka.Topics.AddressMonitorEvent))
		}
	} else {
		logx.Info("Kafka not configured; admin address monitor events disabled")
	}

	return &ServiceContext{
		Config:                              c,
		DB:                                  gormDB,
		RedisClient:                         redisClient,
		AdminUserRepo:                       adminUserRepo,
		AdminLoginLogRepo:                   loginLogRepo,
		AdminAuditLogRepo:                   auditLogRepo,
		SystemConfigRepo:                    systemConfigRepo,
		PasswordHistoryRepo:                 passwordHistoryRepo,
		BackupCodeRepo:                      backupCodeRepo,
		UserRepo:                            userRepo,
		UserAdminNoteRepo:                   userAdminNoteRepo,
		UserWhitelistSettingsRepo:           userWhitelistSettingsRepo,
		User2FAHistoryRepo:                  user2FAHistoryRepo,
		AdminMenuRepo:                       adminMenuRepo,
		AdminPermissionRepo:                 adminPermissionRepo,
		AdminRoleRepo:                       adminRoleRepo,
		AdminRoleMenuRepo:                   adminRoleMenuRepo,
		AdminRolePermissionRepo:             adminRolePermissionRepo,
		AdminUserRoleRepo:                   adminUserRoleRepo,
		AdminRBACRepo:                       adminRBACRepo,
		SessionManager:                      sessionManager,
		TwoFAManager:                        twoFAManager,
		MFAFlow:                             mfaFlow,
		TokenBlacklist:                      tokenBlacklist,
		TokenRefresh:                        tokenRefresh,
		IdempotencyManager:                  idempotencyManager,
		WalletDepositAddressRepo:            walletDepositAddressRepo,
		WalletDepositAddressBalanceRepo:     walletDepositAddressBalanceRepo,
		WalletDepositRepo:                   walletDepositRepo,
		ChainRepo:                           chainRepo,
		CurrencySettingsRepo:                currencySettingsRepo,
		CurrencyChainSettingsRepo:           currencyChainSettingsRepo,
		CurrencyWithdrawFeeRuleRepo:         currencyWithdrawFeeRuleRepo,
		CurrencyGlobalWithdrawFeeRuleRepo:   currencyGlobalWithdrawFeeRuleRepo,
		CurrencyWithdrawAuditRuleRepo:       currencyWithdrawAuditRuleRepo,
		CurrencyGlobalWithdrawAuditRuleRepo: currencyGlobalWithdrawAuditRuleRepo,
		CurrencyGlobalTransferAuditRuleRepo: currencyGlobalTransferAuditRuleRepo,
		CurrencyWithdrawOrderRepo:           currencyWithdrawOrderRepo,
		CurrencyWithdrawOrderEventRepo:      currencyWithdrawOrderEventRepo,
		CurrencyWithdrawPayoutTaskRepo:      currencyWithdrawPayoutTaskRepo,
		CurrencyTransferOrderRepo:           currencyTransferOrderRepo,
		UserTransactionRecordRepo:           userTransactionRecordRepo,
		TransferBatchRepo:                   transferBatchRepo,
		TransferBatchItemRepo:               transferBatchItemRepo,
		Web3UserRepo:                        web3UserRepo,
		Web3UserAddressRepo:                 web3UserAddressRepo,
		Web3TransactionRepo:                 web3TransactionRepo,
		Web3BalanceChangeRepo:               web3BalanceChangeRepo,
		BlacklistAddressRepo:                blacklistAddressRepo,
		VaultNetworkRepo:                    vaultNetworkRepo,
		VaultBalanceRepo:                    vaultBalanceRepo,
		VaultThresholdRepo:                  vaultThresholdRepo,
		VaultAdjustmentRepo:                 vaultAdjustmentRepo,
		VaultAdjustmentApprovalRepo:         vaultAdjustmentApprovalRepo,
		VaultSyncTaskRepo:                   vaultSyncTaskRepo,
		VaultAddressRepo:                    vaultAddressRepo,
		VaultAddressBalanceRepo:             vaultAddressBalanceRepo,
		ChainSyncClient:                     chainSyncClient,
		ChainRpc:                            chainRpcClient,
		SignerRpc:                           signerRpcClient,
		AccountingRpc:                       accountingRpcClient,
		SwapRpc:                             swapRpcClient,
		ConsolidationRpc:                    consolidationRpcClient,
		hotWalletStateCache:                 newHotWalletStateCache(2 * time.Second),
		AlertConfigRepo:                     alertConfigRepo,
		AddressMonitorEventProducer:         addressEventProducer,
	}
}

func (svcCtx *ServiceContext) PublishAddressMonitorEvent(ctx context.Context, evt mq.AddressMonitorEvent) {
	if svcCtx == nil || svcCtx.AddressMonitorEventProducer == nil {
		return
	}

	topic := strings.TrimSpace(svcCtx.Config.Kafka.Topics.AddressMonitorEvent)
	if topic == "" {
		topic = "wallet.address.monitor.events"
	}

	evt.Chain = strings.TrimSpace(evt.Chain)
	evt.Address = strings.TrimSpace(evt.Address)
	if evt.Version == 0 {
		evt.Version = 1
	}
	if evt.EventID == "" {
		evt.EventID = utils.GenerateIDString()
	}
	if evt.CreatedAt.IsZero() {
		evt.CreatedAt = time.Now()
	}

	key := fmt.Sprintf("%s:%s:%s", evt.Source, strings.ToUpper(evt.Chain), evt.Address)
	if _, err := svcCtx.AddressMonitorEventProducer.SendMessage(topic, key, evt); err != nil {
		logx.WithContext(ctx).Errorf("PublishAddressMonitorEvent failed: topic=%s key=%s err=%v", topic, key, err)
	}
}
