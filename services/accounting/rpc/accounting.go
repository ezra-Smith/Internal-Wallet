package main

import (
	"context"
	"flag"
	"fmt"

	"internalwallet/pkg/secrets"
	"internalwallet/proto/pb"
	"internalwallet/services/accounting/rpc/internal/config"
	"internalwallet/services/accounting/rpc/internal/server"
	"internalwallet/services/accounting/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/accounting.yaml", "the config file")

func main() {
	flag.Parse()

	// Step 1: 加载密钥配置
	ctx := context.Background()
	secretConfig, err := secrets.LoadFromConfigMode(ctx, *configFile)
	if err != nil {
		logx.Errorf("Failed to load secrets: %v", err)
		panic(err)
	}
	logx.Infof("Secrets loaded successfully (environment: %s)", secretConfig.Environment)

	// Step 2: 加载服务配置
	var c config.Config
	conf.MustLoad(*configFile, &c)

	// Step 3: 注入密钥到配置
	// IMPORTANT:
	// - 在本地/开发（secrets.Environment=local 等）场景下，secrets loader 会为 Host/Port/Database 提供默认值
	//   若这里无条件覆盖，会把配置文件中正确的 ACCOUNTING_MYSQL_* 覆盖成 localhost 默认值，导致 DB 连接失败。
	// - 因此：仅在从 AWS Secrets（production）加载时，才“完整注入” MySQL 的 host/port/database；
	//   在本地环境只覆盖真正的敏感项（password 等），且仅当 secrets 中有值时才覆盖。
	isAWSSecretsEnv := secretConfig.Environment == "production"
	if isAWSSecretsEnv {
		c.MySQL.Host = secretConfig.Database.MySQL.Host
		c.MySQL.Port = secretConfig.Database.MySQL.Port
		c.MySQL.Username = secretConfig.Database.MySQL.Username
		c.MySQL.Password = secretConfig.Database.MySQL.Password
		c.MySQL.Database = secretConfig.Database.MySQL.Database
	} else {
		if secretConfig.Database.MySQL.Password != "" {
			c.MySQL.Password = secretConfig.Database.MySQL.Password
		}
	}

	// Step 4: 创建服务上下文
	svcCtx := svc.NewServiceContext(c)

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		pb.RegisterAccountingServer(grpcServer, server.NewAccountingServer(svcCtx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	defer s.Stop()

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}
