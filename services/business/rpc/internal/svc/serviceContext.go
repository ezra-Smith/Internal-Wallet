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

	"internalwallet/common/cache"
	"internalwallet/common/captcha"
	commonDB "internalwallet/common/db"
	"internalwallet/common/mq"
	"internalwallet/common/utils"
	"internalwallet/pkg/notify"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/config"
	"internalwallet/services/business/rpc/internal/model"
	"internalwallet/services/business/rpc/internal/repository"
	"internalwallet/services/chainrpc/rpc/chainrpc"
	"internalwallet/services/chainsync/rpc/chainsync"
	"internalwallet/services/swap/rpc/swap"
)

type ServiceContext struct {
	Config        config.Config
	RedisClient   *redis.Client
	AccountCache  *cache.RedisCache
	GeetestClient *captcha.GeetestClient

	SupportedCountryCodes   []string
	SupportedCountryCodeSet map[string]struct{}

	// GORM 相关
	DB                                          *gorm.DB
	UserAccountRepository                       repository.UserAccountRepository
	UserSecuritySettingsRepository              repository.UserSecuritySettingsRepository
	WalletDepositAddressRepository              repository.WalletDepositAddressRepository
	DepositAddressBookRepository                repository.DepositAddressBookRepository
	WalletDepositRepository                     repository.WalletDepositRepository
	UserWalletAddressRepository                 repository.UserWalletAddressRepository
	UserTransactionRecordRepository             repository.UserTransactionRecordRepository
	UserDeviceRepository                        repository.UserDeviceRepository
	UserLoginRecordRepository                   repository.UserLoginRecordRepository
	UserNotificationRepository                  repository.UserNotificationRepository
	UserKycRepository                           repository.UserKycRepository
	UserRoleRepository                          repository.UserRoleRepository
	UserSessionRepository                       repository.UserSessionRepository
	VerificationCodeRepository                  repository.VerificationCodeRepository
	WithdrawalAuditRepository                   repository.WithdrawalAuditRepository
	PayrollRecordRepository                     repository.PayrollRecordRepository
	SwapTransactionRepository                   repository.SwapTransactionRepository
	TransferAuditRepository                     repository.TransferAuditRepository
	LoginLogRepository                          repository.LoginLogRepository
	GeetestValidationLogRepository              repository.GeetestValidationLogRepository
	TrustedDeviceRepository                     repository.TrustedDeviceRepository
	BlacklistRepository                         repository.BlacklistRepository
	SensitiveAddressRepository                  repository.SensitiveAddressRepository
	ChainRepository                             repository.ChainRepository
	CurrencySettingsRepository                  repository.CurrencySettingsRepository
	CurrencyChainSettingsRepository             repository.CurrencyChainSettingsRepository
	CurrencyWithdrawFeeRuleRepository           repository.CurrencyWithdrawFeeRuleRepository
	CurrencyGlobalWithdrawFeeRuleRepository     repository.CurrencyGlobalWithdrawFeeRuleRepository
	CurrencyWithdrawAuditRuleRepository         repository.CurrencyWithdrawAuditRuleRepository
	CurrencyGlobalWithdrawAuditRuleRepository   repository.CurrencyGlobalWithdrawAuditRuleRepository
	CurrencyWithdrawOrderRepository             repository.CurrencyWithdrawOrderRepository
	CurrencyWithdrawOrderEventRepository        repository.CurrencyWithdrawOrderEventRepository
	CurrencyWithdrawPayoutTaskRepository        repository.CurrencyWithdrawPayoutTaskRepository
	LanguageRepository                          repository.LanguageRepository
	PriceUnitRepository                         repository.PriceUnitRepository
	PlatformBindingRepository                   repository.PlatformBindingRepository
	UserWhitelistSettingsRepository             repository.UserWhitelistSettingsRepository
	UserWithdrawAuditWhitelistRuleRepository    repository.UserWithdrawAuditWhitelistRuleRepository
	User2FAHistoryRepository                    repository.User2FAHistoryRepository
	MemberBiometricCredentialRepository         repository.MemberBiometricCredentialRepository
	Web3UserRepository                          repository.Web3UserRepository
	Web3UserAddressRepository                   repository.Web3UserAddressRepository
	Web3UserAddressBalanceRepository            repository.Web3UserAddressBalanceRepository
	Web3BalanceChangeRepository                 repository.Web3BalanceChangeRepository
	Web3TransactionRepository                   repository.Web3TransactionRepository
	MemberInternalAddressRepository             repository.MemberInternalAddressRepository
	CurrencyTransferOrderRepository             repository.CurrencyTransferOrderRepository
	WalletDepositAddressBalanceRepository       repository.WalletDepositAddressBalanceRepository
	WalletDepositAddressBalanceChangeRepository repository.WalletDepositAddressBalanceChangeRepository

	// RPC 客户端
	SignerRpc       pb.SignerServiceClient
	ChainRpc        chainrpc.ChainRPC
	ChainSyncRpc    chainsync.ChainSync
	SwapRpc         swap.Swap
	AdminRpc        pb.AdminClient
	AccountingRpc   pb.AccountingClient
	NotificationRpc pb.NotificationClient

	// Kafka 消费者（用于消费 chainsync 的交易确认和余额变动消息）
	// 注意：KafkaConsumer 不在 ServiceContext 中初始化，避免循环依赖
	// 它在 business.go 中初始化并启动，类型为 interface{} 避免导入 consumer 包
	KafkaConsumer interface{} // *consumer.KafkaConsumer（在 business.go 中设置）

	AddressMonitorEventProducer mq.KafkaProducer
}

func NewServiceContext(c config.Config) *ServiceContext {
	// 初始化雪花ID生成器
	if err := utils.Init(c.NodeID); err != nil {
		logx.Severef("Failed to initialize snowflake ID generator: %v", err)
	}
	logx.Infof("Snowflake ID generator initialized with NodeID: %d", c.NodeID)

	// 初始化邮件配置
	// Gmail 应用专用密码可能包含空格，需要去掉
	password := strings.ReplaceAll(strings.TrimSpace(c.Email.FromPassword), " ", "")
	notify.SetDefaultEmailConfig(notify.EmailConfig{
		SMTPHost:           c.Email.SMTPHost,
		SMTPPort:           c.Email.SMTPPort,
		FromAddress:        c.Email.FromAddress,
		FromPassword:       password,
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
	if c.Email.SMTPHost != "" {
		logx.Infof("✓ Email config initialized: SMTP=%s:%s", c.Email.SMTPHost, c.Email.SMTPPort)
	} else {
		logx.Info("Email SMTP config not found, email sending may fail (unless env vars are set)")
	}

	// 初始化 Redis
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
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		logx.Errorf("Failed to connect to Redis: %v", err)
	} else {
		logx.Info("✓ Connected to Redis for business service")
	}
	accountCache := cache.NewRedisCache(redisClient, "account")

	geetestClient := captcha.NewGeetestClient(captcha.GeetestConfig{
		Enabled:    c.Geetest.Enabled,
		CaptchaID:  c.Geetest.CaptchaID,
		CaptchaKey: c.Geetest.CaptchaKey,
		APIServer:  c.Geetest.APIServer,
		Timeout:    c.Geetest.Timeout,
		FailOpen:   c.Geetest.FailOpen,
	})

	// 可选 GORM 初始化
	var gormDB *gorm.DB
	var userAccountRepo repository.UserAccountRepository
	var userSecuritySettingsRepo repository.UserSecuritySettingsRepository
	var walletDepositAddressRepo repository.WalletDepositAddressRepository
	var depositAddressBookRepo repository.DepositAddressBookRepository
	var walletDepositRepo repository.WalletDepositRepository
	var userWalletAddressRepo repository.UserWalletAddressRepository
	var userTransactionRecordRepo repository.UserTransactionRecordRepository
	var userDeviceRepo repository.UserDeviceRepository
	var userLoginRecordRepo repository.UserLoginRecordRepository
	var userNotificationRepo repository.UserNotificationRepository
	var userKycRepo repository.UserKycRepository
	var userRoleRepo repository.UserRoleRepository
	var userSessionRepo repository.UserSessionRepository
	var verificationCodeRepo repository.VerificationCodeRepository
	var withdrawalAuditRepo repository.WithdrawalAuditRepository
	var payrollRecordRepo repository.PayrollRecordRepository
	var swapTransactionRepo repository.SwapTransactionRepository
	var transferAuditRepo repository.TransferAuditRepository
	var loginLogRepo repository.LoginLogRepository
	var geetestValidationLogRepo repository.GeetestValidationLogRepository
	var trustedDeviceRepo repository.TrustedDeviceRepository
	var blacklistRepo repository.BlacklistRepository
	var chainRepo repository.ChainRepository
	var currencySettingsRepo repository.CurrencySettingsRepository
	var currencyChainSettingsRepo repository.CurrencyChainSettingsRepository
	var currencyWithdrawFeeRuleRepo repository.CurrencyWithdrawFeeRuleRepository
	var currencyGlobalWithdrawFeeRuleRepo repository.CurrencyGlobalWithdrawFeeRuleRepository
	var currencyWithdrawAuditRuleRepo repository.CurrencyWithdrawAuditRuleRepository
	var currencyGlobalWithdrawAuditRuleRepo repository.CurrencyGlobalWithdrawAuditRuleRepository
	var currencyWithdrawOrderRepo repository.CurrencyWithdrawOrderRepository
	var currencyWithdrawOrderEventRepo repository.CurrencyWithdrawOrderEventRepository
	var currencyWithdrawPayoutTaskRepo repository.CurrencyWithdrawPayoutTaskRepository
	var languageRepo repository.LanguageRepository
	var priceUnitRepo repository.PriceUnitRepository
	var platformBindingRepo repository.PlatformBindingRepository
	var userWhitelistSettingsRepo repository.UserWhitelistSettingsRepository
	var userWithdrawAuditWhitelistRuleRepo repository.UserWithdrawAuditWhitelistRuleRepository
	var user2FAHistoryRepo repository.User2FAHistoryRepository
	var memberBiometricCredentialRepo repository.MemberBiometricCredentialRepository
	var sensitiveAddressRepo repository.SensitiveAddressRepository
	var web3UserRepo repository.Web3UserRepository
	var web3UserAddressRepo repository.Web3UserAddressRepository
	var web3UserAddressBalanceRepo repository.Web3UserAddressBalanceRepository
	var web3BalanceChangeRepo repository.Web3BalanceChangeRepository
	var web3TransactionRepo repository.Web3TransactionRepository
	var memberInternalAddressRepo repository.MemberInternalAddressRepository
	var currencyTransferOrderRepo repository.CurrencyTransferOrderRepository
	var walletDepositAddressBalanceRepo repository.WalletDepositAddressBalanceRepository
	var walletDepositAddressBalanceChangeRepo repository.WalletDepositAddressBalanceChangeRepository
	if c.MySQL.Database != "" {
		var err error
		gormDB, err = commonDB.InitGorm(c.MySQL)
		if err != nil {
			logx.Errorf("Failed to initialize GORM: %v", err)
			logx.Info("Service will continue with sqlx only")
		} else {
			logx.Info("✓ GORM initialized successfully")
			userAccountRepo = repository.NewUserAccountRepository(gormDB)
			logx.Info("✓ UserAccountRepository initialized successfully")
			userSecuritySettingsRepo = repository.NewUserSecuritySettingsRepository(gormDB)
			walletDepositAddressRepo = repository.NewWalletDepositAddressRepository(gormDB)
			depositAddressBookRepo = repository.NewDepositAddressBookRepository(gormDB)
			walletDepositRepo = repository.NewWalletDepositRepository(gormDB)
			userWalletAddressRepo = repository.NewUserWalletAddressRepository(gormDB)
			userTransactionRecordRepo = repository.NewUserTransactionRecordRepository(gormDB)
			userDeviceRepo = repository.NewUserDeviceRepository(gormDB)
			userLoginRecordRepo = repository.NewUserLoginRecordRepository(gormDB)
			userNotificationRepo = repository.NewUserNotificationRepository(gormDB)
			userKycRepo = repository.NewUserKycRepository(gormDB)
			userRoleRepo = repository.NewUserRoleRepository(gormDB)
			userSessionRepo = repository.NewUserSessionRepository(gormDB)
			verificationCodeRepo = repository.NewVerificationCodeRepository(gormDB)
			withdrawalAuditRepo = repository.NewWithdrawalAuditRepository(gormDB)
			payrollRecordRepo = repository.NewPayrollRecordRepository(gormDB)
			swapTransactionRepo = repository.NewSwapTransactionRepository(gormDB)
			transferAuditRepo = repository.NewTransferAuditRepository(gormDB)
			loginLogRepo = repository.NewLoginLogRepository(gormDB)
			geetestValidationLogRepo = repository.NewGeetestValidationLogRepository(gormDB)
			trustedDeviceRepo = repository.NewTrustedDeviceRepository(gormDB)
			blacklistRepo = repository.NewBlacklistRepository(gormDB)
			sensitiveAddressRepo = repository.NewSensitiveAddressRepository(gormDB)
			chainRepo = repository.NewChainRepository(gormDB)
			currencySettingsRepo = repository.NewCurrencySettingsRepository(gormDB)
			currencyChainSettingsRepo = repository.NewCurrencyChainSettingsRepository(gormDB)
			currencyWithdrawFeeRuleRepo = repository.NewCurrencyWithdrawFeeRuleRepository(gormDB)
			currencyGlobalWithdrawFeeRuleRepo = repository.NewCurrencyGlobalWithdrawFeeRuleRepository(gormDB)
			currencyWithdrawAuditRuleRepo = repository.NewCurrencyWithdrawAuditRuleRepository(gormDB)
			currencyGlobalWithdrawAuditRuleRepo = repository.NewCurrencyGlobalWithdrawAuditRuleRepository(gormDB)
			currencyWithdrawOrderRepo = repository.NewCurrencyWithdrawOrderRepository(gormDB)
			currencyWithdrawOrderEventRepo = repository.NewCurrencyWithdrawOrderEventRepository(gormDB)
			currencyWithdrawPayoutTaskRepo = repository.NewCurrencyWithdrawPayoutTaskRepository(gormDB)
			languageRepo = repository.NewLanguageRepository(gormDB)
			priceUnitRepo = repository.NewPriceUnitRepository(gormDB)
			platformBindingRepo = repository.NewPlatformBindingRepository(gormDB)
			userWhitelistSettingsRepo = repository.NewUserWhitelistSettingsRepository(gormDB)
			userWithdrawAuditWhitelistRuleRepo = repository.NewUserWithdrawAuditWhitelistRuleRepository(gormDB)
			user2FAHistoryRepo = repository.NewUser2FAHistoryRepository(gormDB)
			memberBiometricCredentialRepo = repository.NewMemberBiometricCredentialRepository(gormDB)
			web3UserRepo = repository.NewWeb3UserRepository(gormDB)
			web3UserAddressRepo = repository.NewWeb3UserAddressRepository(gormDB)
			web3UserAddressBalanceRepo = repository.NewWeb3UserAddressBalanceRepository(gormDB)
			web3BalanceChangeRepo = repository.NewWeb3BalanceChangeRepository(gormDB)
			web3TransactionRepo = repository.NewWeb3TransactionRepository(gormDB)
			memberInternalAddressRepo = repository.NewMemberInternalAddressRepository(gormDB)
			currencyTransferOrderRepo = repository.NewCurrencyTransferOrderRepository(gormDB)
			walletDepositAddressBalanceRepo = repository.NewWalletDepositAddressBalanceRepository(gormDB)
			walletDepositAddressBalanceChangeRepo = repository.NewWalletDepositAddressBalanceChangeRepository(gormDB)
			if err := gormDB.AutoMigrate(
				&model.UserModel{},
				&model.UserRoleModel{},
				&model.UserSessionModel{},
				&model.UserSecuritySettingsModel{},
				&model.UserWhitelistSettingsModel{},
				&model.UserWithdrawAuditWhitelistRuleModel{},
				&model.User2FAHistoryModel{},
				&model.MemberBiometricCredentialModel{},
				&model.UserDeviceModel{},
				&model.UserLoginRecordModel{},
				&model.LoginLogModel{},
				&model.GeetestValidationLogModel{},
				&model.TrustedDeviceModel{},
				&model.UserNotificationModel{},
				&model.VerificationCodeModel{},
				&model.BlacklistModel{},
				&model.ChainModel{},
				&model.CurrencySettingsModel{},
				&model.CurrencyChainSettingsModel{},
				&model.CurrencyWithdrawFeeRuleModel{},
				&model.CurrencyGlobalWithdrawFeeRuleModel{},
				&model.CurrencyWithdrawAuditRuleModel{},
				&model.CurrencyGlobalWithdrawAuditRuleModel{},
				&model.CurrencyWithdrawOrderModel{},
				&model.LanguageModel{},
				&model.PriceUnitModel{},
				&model.UserWalletAddressModel{},
				&model.PlatformBindingModel{},
				&model.WithdrawalAuditModel{},
				&model.PayrollRecordModel{},
				&model.SwapTransactionModel{},
				&model.TransferAuditModel{},
				&model.UserKycModel{},
				&model.Web3UserModel{},
				&model.Web3UserAddressModel{},
				&model.Web3UserAddressBalanceModel{},
				&model.Web3BalanceChangeModel{},
				&model.Web3TransactionModel{},
				&model.MemberInternalAddressModel{},
				&model.WalletDepositAddressBalanceModel{},
				&model.WalletDepositAddressBalanceChangeModel{},
			); err != nil {
				logx.Errorf("AutoMigrate failed: %v", err)
			} else {
				logx.Info("✓ AutoMigrate completed for business models")
			}
		}
	} else {
		logx.Info("GORM MySQL config not found, using sqlx only")
	}

	// 初始化Signer RPC客户端（可选）
	var signerRpc pb.SignerServiceClient
	if len(c.SignerRpc.Etcd.Hosts) > 0 || len(c.SignerRpc.Endpoints) > 0 {
		signerConn, err := zrpc.NewClient(c.SignerRpc)
		if err != nil {
			logx.Errorf("Failed to connect to SignerRpc: %v (service may not be started yet)", err)
		} else {
			signerRpc = pb.NewSignerServiceClient(signerConn.Conn())
			logx.Info("✓ SignerRpc client initialized successfully")
		}
	} else {
		logx.Info("SignerRpc config not found, address generation will be disabled")
	}

	var chainRpcClient chainrpc.ChainRPC
	if len(c.ChainRpc.Etcd.Hosts) > 0 || len(c.ChainRpc.Endpoints) > 0 {
		chainConn, err := zrpc.NewClient(c.ChainRpc)
		if err != nil {
			logx.Errorf("Failed to connect to ChainRpc: %v (service may not be started yet)", err)
		} else {
			chainRpcClient = chainrpc.NewChainRPC(chainConn)
			logx.Info("✓ ChainRpc client initialized successfully")
		}
	} else {
		logx.Info("ChainRpc config not found, transfer will be disabled")
	}

	// ChainSync RPC 客户端（可选；用于地址监控）
	var chainSyncRpcClient chainsync.ChainSync
	if len(c.ChainSyncRpc.Etcd.Hosts) > 0 || len(c.ChainSyncRpc.Endpoints) > 0 {
		chainSyncConn, err := zrpc.NewClient(c.ChainSyncRpc)
		if err != nil {
			logx.Errorf("Failed to connect to ChainSyncRpc: %v (service may not be started yet)", err)
		} else {
			chainSyncRpcClient = chainsync.NewChainSync(chainSyncConn)
			logx.Info("✓ ChainSyncRpc client initialized successfully")
		}
	} else {
		logx.Info("ChainSyncRpc config not found, address monitoring will be disabled")
	}

	// Swap RPC 客户端（可选；用于 Web3 swap 聚合器）
	logx.Infof("=== [BUSINESS-SVC] Initializing SwapRpc client ===")
	logx.Infof("=== [BUSINESS-SVC] SwapRpc.Etcd.Hosts: %v ===", c.SwapRpc.Etcd.Hosts)
	logx.Infof("=== [BUSINESS-SVC] SwapRpc.Endpoints: %v ===", c.SwapRpc.Endpoints)
	logx.Infof("=== [BUSINESS-SVC] SwapRpc.Target: %s ===", c.SwapRpc.Target)
	logx.Infof("=== [BUSINESS-SVC] SwapAuth.ApiKey configured: %v ===", c.SwapAuth.ApiKey != "")

	var swapRpcClient swap.Swap
	if len(c.SwapRpc.Etcd.Hosts) > 0 || len(c.SwapRpc.Endpoints) > 0 {
		logx.Infof("=== [BUSINESS-SVC] Attempting to connect to SwapRpc... ===")
		swapConn, err := zrpc.NewClient(
			c.SwapRpc,
			zrpc.WithUnaryClientInterceptor(swapClientInterceptor(c.SwapAuth.ApiKey)),
		)
		if err != nil {
			logx.Errorf("=== [BUSINESS-SVC] Failed to connect to SwapRpc: %v ===", err)
			logx.Errorf("=== [BUSINESS-SVC] SwapRpc service may not be started yet ===")
		} else {
			swapRpcClient = swap.NewSwap(swapConn)
			logx.Info("✓ SwapRpc client initialized successfully")
			logx.Infof("=== [BUSINESS-SVC] SwapRpc client type: %T ===", swapRpcClient)
		}
	} else {
		logx.Info("=== [BUSINESS-SVC] SwapRpc config not found, swap features will be disabled ===")
	}

	// Admin RPC 客户端（可选；用于用户钱包初始化）
	var adminRpcClient pb.AdminClient
	if len(c.AdminRpc.Etcd.Hosts) > 0 || len(c.AdminRpc.Endpoints) > 0 {
		adminConn, err := zrpc.NewClient(c.AdminRpc)
		if err != nil {
			logx.Errorf("Failed to connect to AdminRpc: %v (service may not be started yet)", err)
		} else {
			adminRpcClient = pb.NewAdminClient(adminConn.Conn())
			logx.Info("✓ AdminRpc client initialized successfully")
		}
	} else {
		logx.Info("AdminRpc config not found, wallet initialization will be disabled")
	}

	// Accounting RPC 客户端（可选；用于账本/余额写入）
	var accountingRpcClient pb.AccountingClient
	if len(c.AccountingRpc.Etcd.Hosts) > 0 || len(c.AccountingRpc.Endpoints) > 0 {
		accConn, err := zrpc.NewClient(c.AccountingRpc)
		if err != nil {
			logx.Errorf("Failed to connect to AccountingRpc: %v (service may not be started yet)", err)
		} else {
			accountingRpcClient = pb.NewAccountingClient(accConn.Conn())
			logx.Info("✓ AccountingRpc client initialized successfully")
		}
	} else {
		logx.Info("AccountingRpc config not found, accounting integration will be disabled")
	}

	// Notification RPC 客户端（可选；用于推送通知）
	var notificationRpcClient pb.NotificationClient
	if len(c.NotificationRpc.Etcd.Hosts) > 0 || len(c.NotificationRpc.Endpoints) > 0 {
		notificationConn, err := zrpc.NewClient(c.NotificationRpc)
		if err != nil {
			logx.Errorf("Failed to connect to NotificationRpc: %v (service may not be started yet)", err)
		} else {
			notificationRpcClient = pb.NewNotificationClient(notificationConn.Conn())
			logx.Info("✓ NotificationRpc client initialized successfully")
		}
	} else {
		logx.Info("NotificationRpc config not found, push notification integration will be disabled")
	}

	supportedCountryCodes, supportedCountryCodeSet := buildCountryCodeSet(c.CountryCodes.Supported)
	logx.Infof("✓ Supported country codes: %v", supportedCountryCodes)

	var addressEventProducer mq.KafkaProducer
	brokers := c.KafkaProducer.Brokers
	if len(brokers) == 0 {
		brokers = c.KafkaConsumer.Brokers
	}
	if len(brokers) > 0 {
		username := strings.TrimSpace(c.KafkaProducer.Username)
		password := strings.TrimSpace(c.KafkaProducer.Password)
		security := strings.TrimSpace(c.KafkaProducer.Security)
		saslMech := strings.TrimSpace(c.KafkaProducer.SASLMech)
		if username == "" {
			username = strings.TrimSpace(c.KafkaConsumer.Username)
		}
		if password == "" {
			password = strings.TrimSpace(c.KafkaConsumer.Password)
		}
		if security == "" {
			security = strings.TrimSpace(c.KafkaConsumer.Security)
		}
		if saslMech == "" {
			saslMech = strings.TrimSpace(c.KafkaConsumer.SASLMech)
		}

		p, err := mq.NewSaramaProducer(mq.KafkaProducerConfig{
			Brokers:     brokers,
			Username:    username,
			Password:    password,
			Security:    security,
			SASLMech:    saslMech,
			ClientID:    c.KafkaProducer.ClientID,
			Compression: c.KafkaProducer.Compression,
			MaxRetries:  c.KafkaProducer.MaxRetries,
			Timeout:     time.Duration(c.KafkaProducer.Timeout) * time.Second,
			UseAsync:    c.KafkaProducer.UseAsync,
		})
		if err != nil {
			logx.Errorf("Failed to init Kafka producer (business address monitor events): %v", err)
		} else {
			addressEventProducer = p
			logx.Infof("✓ Kafka producer initialized (business address monitor events): brokers=%v topic=%s", brokers, strings.TrimSpace(c.KafkaProducer.Topics.AddressMonitorEvent))
		}
	} else {
		logx.Info("KafkaProducer not configured; business address monitor events disabled")
	}

	return &ServiceContext{
		Config:                  c,
		RedisClient:             redisClient,
		AccountCache:            accountCache,
		GeetestClient:           geetestClient,
		SupportedCountryCodes:   supportedCountryCodes,
		SupportedCountryCodeSet: supportedCountryCodeSet,
		DB:                      gormDB,

		UserAccountRepository:                       userAccountRepo,
		UserSecuritySettingsRepository:              userSecuritySettingsRepo,
		WalletDepositAddressRepository:              walletDepositAddressRepo,
		DepositAddressBookRepository:                depositAddressBookRepo,
		WalletDepositRepository:                     walletDepositRepo,
		UserWalletAddressRepository:                 userWalletAddressRepo,
		UserTransactionRecordRepository:             userTransactionRecordRepo,
		UserDeviceRepository:                        userDeviceRepo,
		UserLoginRecordRepository:                   userLoginRecordRepo,
		UserNotificationRepository:                  userNotificationRepo,
		UserKycRepository:                           userKycRepo,
		UserRoleRepository:                          userRoleRepo,
		UserSessionRepository:                       userSessionRepo,
		VerificationCodeRepository:                  verificationCodeRepo,
		WithdrawalAuditRepository:                   withdrawalAuditRepo,
		PayrollRecordRepository:                     payrollRecordRepo,
		SwapTransactionRepository:                   swapTransactionRepo,
		TransferAuditRepository:                     transferAuditRepo,
		LoginLogRepository:                          loginLogRepo,
		GeetestValidationLogRepository:              geetestValidationLogRepo,
		TrustedDeviceRepository:                     trustedDeviceRepo,
		BlacklistRepository:                         blacklistRepo,
		SensitiveAddressRepository:                  sensitiveAddressRepo,
		ChainRepository:                             chainRepo,
		CurrencySettingsRepository:                  currencySettingsRepo,
		CurrencyChainSettingsRepository:             currencyChainSettingsRepo,
		CurrencyWithdrawFeeRuleRepository:           currencyWithdrawFeeRuleRepo,
		CurrencyGlobalWithdrawFeeRuleRepository:     currencyGlobalWithdrawFeeRuleRepo,
		CurrencyWithdrawAuditRuleRepository:         currencyWithdrawAuditRuleRepo,
		CurrencyGlobalWithdrawAuditRuleRepository:   currencyGlobalWithdrawAuditRuleRepo,
		CurrencyWithdrawOrderRepository:             currencyWithdrawOrderRepo,
		CurrencyWithdrawOrderEventRepository:        currencyWithdrawOrderEventRepo,
		CurrencyWithdrawPayoutTaskRepository:        currencyWithdrawPayoutTaskRepo,
		LanguageRepository:                          languageRepo,
		PriceUnitRepository:                         priceUnitRepo,
		PlatformBindingRepository:                   platformBindingRepo,
		UserWhitelistSettingsRepository:             userWhitelistSettingsRepo,
		UserWithdrawAuditWhitelistRuleRepository:    userWithdrawAuditWhitelistRuleRepo,
		User2FAHistoryRepository:                    user2FAHistoryRepo,
		MemberBiometricCredentialRepository:         memberBiometricCredentialRepo,
		Web3UserRepository:                          web3UserRepo,
		Web3UserAddressRepository:                   web3UserAddressRepo,
		Web3UserAddressBalanceRepository:            web3UserAddressBalanceRepo,
		Web3BalanceChangeRepository:                 web3BalanceChangeRepo,
		Web3TransactionRepository:                   web3TransactionRepo,
		MemberInternalAddressRepository:             memberInternalAddressRepo,
		CurrencyTransferOrderRepository:             currencyTransferOrderRepo,
		WalletDepositAddressBalanceRepository:       walletDepositAddressBalanceRepo,
		WalletDepositAddressBalanceChangeRepository: walletDepositAddressBalanceChangeRepo,

		SignerRpc:                   signerRpc,
		ChainRpc:                    chainRpcClient,
		ChainSyncRpc:                chainSyncRpcClient,
		SwapRpc:                     swapRpcClient,
		AdminRpc:                    adminRpcClient,
		AccountingRpc:               accountingRpcClient,
		NotificationRpc:             notificationRpcClient,
		AddressMonitorEventProducer: addressEventProducer,
		// KafkaConsumer 将在 business.go 中初始化，避免循环依赖
	}
}

func (svcCtx *ServiceContext) PublishAddressMonitorEvent(ctx context.Context, evt mq.AddressMonitorEvent) {
	if svcCtx == nil || svcCtx.AddressMonitorEventProducer == nil {
		return
	}

	topic := strings.TrimSpace(svcCtx.Config.KafkaProducer.Topics.AddressMonitorEvent)
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
