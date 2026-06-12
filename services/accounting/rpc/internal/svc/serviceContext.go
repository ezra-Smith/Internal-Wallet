package svc

import (
	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"

	commonDB "internalwallet/common/db"
	"internalwallet/common/utils"
	"internalwallet/services/accounting/rpc/internal/config"
	"internalwallet/services/accounting/rpc/internal/repository"
)

type ServiceContext struct {
	Config config.Config

	DB *gorm.DB

	AssetRepo       repository.AssetRepository
	AccountTypeRepo repository.AccountTypeRepository
	AccountRepo     repository.AccountRepository
	BalanceRepo     repository.BalanceRepository
	LedgerRepo      repository.LedgerRepository
	UserTxRepo      repository.UserTransactionRecordRepository
}

func NewServiceContext(c config.Config) *ServiceContext {
	// 初始化雪花ID生成器（必须）
	if err := utils.Init(c.NodeID); err != nil {
		logx.Severef("Failed to initialize snowflake ID generator: %v", err)
	}
	logx.Infof("Snowflake ID generator initialized with NodeID: %d", c.NodeID)

	var gormDB *gorm.DB
	if c.MySQL.Database != "" {
		db, err := commonDB.InitGorm(c.MySQL)
		if err != nil {
			logx.Errorf("Failed to initialize GORM: %v", err)
		} else {
			gormDB = db
			logx.Info("✓ GORM initialized successfully (accounting)")
		}
	}

	var assetRepo repository.AssetRepository
	var accountTypeRepo repository.AccountTypeRepository
	var accountRepo repository.AccountRepository
	var balanceRepo repository.BalanceRepository
	var ledgerRepo repository.LedgerRepository
	var userTxRepo repository.UserTransactionRecordRepository
	if gormDB != nil {
		assetRepo = repository.NewAssetRepository(gormDB)
		accountTypeRepo = repository.NewAccountTypeRepository(gormDB)
		accountRepo = repository.NewAccountRepository(gormDB)
		balanceRepo = repository.NewBalanceRepository(gormDB)
		ledgerRepo = repository.NewLedgerRepository(gormDB)
		userTxRepo = repository.NewUserTransactionRecordRepository(gormDB)
	}

	return &ServiceContext{
		Config:          c,
		DB:              gormDB,
		AssetRepo:       assetRepo,
		AccountTypeRepo: accountTypeRepo,
		AccountRepo:     accountRepo,
		BalanceRepo:     balanceRepo,
		LedgerRepo:      ledgerRepo,
		UserTxRepo:      userTxRepo,
	}
}
