package svc

import (
	"fmt"
	"internalwallet/common/db"
	"internalwallet/common/utils"
	"internalwallet/services/notification/rpc/internal/config"
	"internalwallet/services/notification/rpc/internal/jpush"
	"internalwallet/services/notification/rpc/internal/model"
	"internalwallet/services/notification/rpc/internal/repository"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type ServiceContext struct {
	Config config.Config

	// 数据库
	DB *gorm.DB

	// 仓储层
	DeviceRepo   repository.DeviceRepository
	RecordRepo   repository.RecordRepository
	TemplateRepo repository.TemplateRepository
	SettingsRepo repository.SettingsRepository

	// 极光推送客户端
	JPushClient *jpush.Client
}

func NewServiceContext(c config.Config) *ServiceContext {
	// 1. 初始化数据库连接
	gormDB, err := db.InitGorm(db.MySQLConfig{
		Host:            c.MySQL.Host,
		Port:            c.MySQL.Port,
		Username:        c.MySQL.Username,
		Password:        c.MySQL.Password,
		Database:        c.MySQL.Database,
		MaxIdleConns:    c.MySQL.MaxIdleConns,
		MaxOpenConns:    c.MySQL.MaxOpenConns,
		ConnMaxLifetime: c.MySQL.ConnMaxLifetime,
		LogLevel:        c.MySQL.LogLevel,
		SlowThreshold:   c.MySQL.SlowThreshold,
	})
	if err != nil {
		logx.Severef("Failed to connect to database: %v", err)
		panic(err)
	}

	// 2. 自动迁移表结构（开发环境）
	if c.Mode == "dev" || c.Mode == "test" {
		if err := gormDB.AutoMigrate(
			&model.NotificationDevice{},
			&model.NotificationTemplate{},
			&model.NotificationRecord{},
			&model.NotificationUserSettings{},
		); err != nil {
			logx.Errorf("Failed to auto migrate tables: %v", err)
		} else {
			logx.Info("Database tables migrated successfully")
		}
	}

	// 3. 初始化雪花ID生成器
	if c.NodeID > 0 {
		if err := utils.Init(c.NodeID); err != nil {
			logx.Errorf("Failed to initialize Snowflake: %v", err)
		} else {
			logx.Infof("Snowflake ID generator initialized with NodeID: %d", c.NodeID)
		}
	} else {
		logx.Error("NodeID not configured, using default (0)")
		utils.Init(0) // 使用默认值0
	}

	// 4. 初始化仓储层
	deviceRepo := repository.NewDeviceRepository(gormDB)
	recordRepo := repository.NewRecordRepository(gormDB)
	templateRepo := repository.NewTemplateRepository(gormDB)
	settingsRepo := repository.NewSettingsRepository(gormDB)

	// 5. 初始化极光推送客户端
	jpushClient, err := jpush.NewClient(
		c.JPush.AppKey,
		c.JPush.MasterSecret,
		&jpush.ClientOptions{
			Timeout:        time.Duration(c.JPush.Timeout) * time.Millisecond,
			MaxRetries:     c.JPush.MaxRetries,
			RetryInterval:  time.Duration(c.JPush.RetryInterval) * time.Millisecond,
			ApnsProduction: c.JPush.ApnsProduction,
		},
	)
	if err != nil {
		logx.Severef("Failed to create JPush client: %v", err)
		panic(err)
	}

	// 设置日志器
	jpushClient.SetLogger(logx.WithContext(nil))

	logx.Info("Notification service context initialized successfully")

	return &ServiceContext{
		Config:       c,
		DB:           gormDB,
		DeviceRepo:   deviceRepo,
		RecordRepo:   recordRepo,
		TemplateRepo: templateRepo,
		SettingsRepo: settingsRepo,
		JPushClient:  jpushClient,
	}
}

// Close 关闭服务上下文
func (s *ServiceContext) Close() error {
	if s.DB != nil {
		sqlDB, err := s.DB.DB()
		if err != nil {
			return fmt.Errorf("failed to get sql.DB: %w", err)
		}
		return sqlDB.Close()
	}
	return nil
}
