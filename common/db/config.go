package db

// MySQLConfig GORM MySQL/MariaDB 配置（通用）
// 所有 RPC 服务可以复用这个配置结构
// 注意：MariaDB 完全兼容 MySQL 协议，使用此配置即可
type MySQLConfig struct {
	Host            string `json:",default=127.0.0.1"`
	Port            int    `json:",default=3306"`
	Username        string `json:",default=root"`
	Password        string
	Database        string
	MaxIdleConns    int `json:",default=10"`   // 最大空闲连接数
	MaxOpenConns    int `json:",default=100"`  // 最大打开连接数
	ConnMaxLifetime int `json:",default=3600"` // 连接最大生命周期（秒）
	LogLevel        int `json:",default=1"`    // 日志级别: 1=Silent 2=Error 3=Warn 4=Info
	SlowThreshold   int `json:",default=200"`  // 慢查询阈值（毫秒）
}
