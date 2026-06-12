package db

import (
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// InitGorm 初始化 GORM 数据库连接（通用方法）
// 支持 MySQL 和 MariaDB（MariaDB 完全兼容 MySQL 协议）
// 所有 RPC 服务可以直接调用这个方法
func InitGorm(c MySQLConfig) (*gorm.DB, error) {
	// 构建 DSN
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		c.Username,
		c.Password,
		c.Host,
		c.Port,
		c.Database,
	)

	// 配置 GORM
	gormConfig := &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true, // 使用单数表名
		},
		Logger: logger.Default.LogMode(logger.LogLevel(c.LogLevel)),
		NowFunc: func() time.Time {
			return time.Now().Local()
		},
		// 禁用外键约束（根据需要开启）
		DisableForeignKeyConstraintWhenMigrating: true,
	}

	// 如果设置了慢查询阈值，配置自定义 Logger
	if c.SlowThreshold > 0 {
		gormConfig.Logger = logger.New(
			&gormLogger{},
			logger.Config{
				SlowThreshold:             time.Duration(c.SlowThreshold) * time.Millisecond,
				LogLevel:                  logger.LogLevel(c.LogLevel),
				IgnoreRecordNotFoundError: true,
				Colorful:                  false,
			},
		)
	}

	// 连接数据库
	db, err := gorm.Open(mysql.Open(dsn), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect database: %w", err)
	}

	// 获取底层的 *sql.DB
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB: %w", err)
	}

	// 设置连接池参数
	sqlDB.SetMaxIdleConns(c.MaxIdleConns)
	sqlDB.SetMaxOpenConns(c.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(time.Duration(c.ConnMaxLifetime) * time.Second)

	// 测试连接
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logx.Infof("✓ GORM connected to %s:%d/%s", c.Host, c.Port, c.Database)
	return db, nil
}

// gormLogger 自定义日志实现，使用 go-zero 的 logx
type gormLogger struct{}

func (l *gormLogger) Printf(format string, args ...interface{}) {
	logx.Infof("[GORM] "+format, args...)
}
