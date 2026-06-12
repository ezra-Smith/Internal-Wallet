package main

import (
	"context"
	"flag"
	"fmt"

	"internalwallet/common/interceptor"
	"internalwallet/pkg/secrets"
	"internalwallet/proto/pb"
	"internalwallet/services/notification/rpc/internal/config"
	"internalwallet/services/notification/rpc/internal/server"
	"internalwallet/services/notification/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/notification.yaml", "the config file")

func main() {
	flag.Parse()

	// Step 1: 加载密钥配置（优先级最高）
	ctx := context.Background()

	// 从配置文件 Mode 自动读取
	secretConfig, err := secrets.LoadFromConfigMode(ctx, *configFile)
	if err != nil {
		logx.Errorf("Failed to load secrets: %v", err)
		panic(err)
	}
	logx.Infof("Secrets loaded successfully (environment: %s)", secretConfig.Environment)

	// Step 2: 加载服务配置（YAML）
	var c config.Config
	conf.MustLoad(*configFile, &c)

	// Step 3: 注入密钥到配置（覆盖YAML中的占位符）
	isAWSSecretsEnv := secretConfig.Environment == "production"
	if isAWSSecretsEnv {
		// MySQL 配置（完整注入）
		c.MySQL.Host = secretConfig.Database.MySQL.Host
		c.MySQL.Port = secretConfig.Database.MySQL.Port
		c.MySQL.Username = secretConfig.Database.MySQL.Username
		c.MySQL.Password = secretConfig.Database.MySQL.Password
		c.MySQL.Database = secretConfig.Database.MySQL.Database
	} else {
		// Local/dev：只覆盖密码（避免覆盖 host/port/database）
		if secretConfig.Database.MySQL.Password != "" {
			c.MySQL.Password = secretConfig.Database.MySQL.Password
		}
	}

	// Redis 配置
	if len(c.CacheRedis) > 0 {
		if isAWSSecretsEnv {
			c.CacheRedis[0].Host = secretConfig.Cache.Redis.Address()
			c.CacheRedis[0].Pass = secretConfig.Cache.Redis.Password
		} else {
			if secretConfig.Cache.Redis.Password != "" {
				c.CacheRedis[0].Pass = secretConfig.Cache.Redis.Password
			}
		}
	}

	// 极光推送配置
	if isAWSSecretsEnv || secretConfig.Notifications.JPush.AppKey != "" {
		c.JPush.AppKey = secretConfig.Notifications.JPush.AppKey
		c.JPush.MasterSecret = secretConfig.Notifications.JPush.MasterSecret
		c.JPush.ApnsProduction = secretConfig.Notifications.JPush.ApnsProduction
	}

	// ========================================
	// Step 4: 创建服务上下文
	// ========================================
	svcCtx := svc.NewServiceContext(c)
	defer func() {
		if err := svcCtx.Close(); err != nil {
			logx.Errorf("Failed to close service context: %v", err)
		}
	}()

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		pb.RegisterNotificationServer(grpcServer, server.NewNotificationServer(svcCtx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})

	// 添加服务端元数据拦截器（接收 user_id、email 等信息）
	s.AddUnaryInterceptors(interceptor.ServerMetadataUnaryInterceptor())

	defer s.Stop()

	fmt.Printf("Starting notification rpc server at %s...\n", c.ListenOn)
	s.Start()
}
