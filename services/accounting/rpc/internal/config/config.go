package config

import (
	commonDB "internalwallet/common/db"

	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf

	// 雪花ID配置（必须配置，范围0-1023，每个服务唯一）
	NodeID int64 `json:",optional"`

	// GORM MySQL/MariaDB 配置（统一使用 common/db）
	MySQL commonDB.MySQLConfig `json:",optional"`
}
