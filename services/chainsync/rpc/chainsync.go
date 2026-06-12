package main

import (
	"context"
	"flag"
	"fmt"
	"os/signal"
	"syscall"

	"internalwallet/pkg/secrets"
	"internalwallet/services/chainsync/rpc/internal/config"
	server "internalwallet/services/chainsync/rpc/internal/server"
	"internalwallet/services/chainsync/rpc/internal/svc"

	"internalwallet/proto/pb"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/chainsync.yaml", "the config file")

func main() {
	flag.Parse()

	// Step 1: 加载密钥配置（优先级最高）
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// 从配置文件 Mode 自动读取（推荐，无需配置环境变量）
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
	// MySQL 配置（完整注入）
	c.MySQL.Host = secretConfig.Database.MySQL.Host
	c.MySQL.Port = secretConfig.Database.MySQL.Port
	c.MySQL.Username = secretConfig.Database.MySQL.Username
	c.MySQL.Password = secretConfig.Database.MySQL.Password
	c.MySQL.Database = secretConfig.Database.MySQL.Database

	// Redis 配置
	if len(c.CacheRedis) > 0 {
		c.CacheRedis[0].Host = secretConfig.Cache.Redis.Address()
		c.CacheRedis[0].Pass = secretConfig.Cache.Redis.Password
	}

	// Kafka 配置（如果有密码）
	if secretConfig.Messaging.Kafka.Password != "" {
		c.Kafka.Password = secretConfig.Messaging.Kafka.Password
	}

	// Provider API keys: load from SecretConfig (preferred in AWS).
	// This mirrors env overrides but avoids requiring env vars in k8s (CSI mount only).
	if secretConfig.ChainSync.EthereumApiKey != "" {
		c.Providers.Ethereum.APIKey = secretConfig.ChainSync.EthereumApiKey
	}
	if secretConfig.ChainSync.QuickNodeApiKey != "" {
		c.Providers.QuickNode.APIKey = secretConfig.ChainSync.QuickNodeApiKey
	}
	if secretConfig.ChainSync.InfuraApiKey != "" {
		c.Providers.Infura.APIKey = secretConfig.ChainSync.InfuraApiKey
	}
	if secretConfig.ChainSync.AlchemyApiKey != "" {
		c.Providers.Alchemy.APIKey = secretConfig.ChainSync.AlchemyApiKey
	}
	if secretConfig.ChainSync.BscApiKey != "" {
		c.Providers.BSC.APIKey = secretConfig.ChainSync.BscApiKey
	}
	if secretConfig.ChainSync.TronApiKey != "" {
		c.Providers.Tron.APIKey = secretConfig.ChainSync.TronApiKey
	}
	if secretConfig.ChainSync.SelfHostedApiKey != "" {
		for i := range c.Providers.SelfHosted {
			if c.Providers.SelfHosted[i].APIKey == "" {
				c.Providers.SelfHosted[i].APIKey = secretConfig.ChainSync.SelfHostedApiKey
			}
		}
	}

	// Step 3.1: 环境变量覆盖（优先级最高）
	config.LoadEnvVariables(&c)
	if err := c.Validate(); err != nil {
		logx.Errorf("Invalid chainsync config: %v", err)
		panic(err)
	}

	// ========================================
	// Step 4: 创建服务上下文（原有逻辑）
	// ========================================
	svcCtx := svc.NewServiceContext(c)
	if err := svcCtx.Start(ctx); err != nil {
		logx.Errorf("Failed to start chainsync components: %v", err)
		panic(err)
	}
	defer svcCtx.Stop()

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		pb.RegisterChainSyncServer(grpcServer, server.NewChainSyncServer(svcCtx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})

	go func() {
		<-ctx.Done()
		logx.Info("Received shutdown signal, stopping rpc server...")
		s.Stop()
	}()

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}
