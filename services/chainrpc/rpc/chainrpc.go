package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"internalwallet/common/utils"
	"internalwallet/pkg/secrets"
	"internalwallet/proto/pb"
	"internalwallet/services/chainrpc/rpc/internal/config"
	"internalwallet/services/chainrpc/rpc/internal/server"
	"internalwallet/services/chainrpc/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/chainrpc.yaml", "the config file")

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
	// 根据环境决定注入策略：AWS Secrets 环境完整注入，Local 环境只覆盖密码
	isAWSSecretsEnv := secretConfig.Environment == "production"

	// Redis 配置（使用 Address() 获取 "host:port" 格式）
	if len(c.CacheRedis) > 0 {
		if isAWSSecretsEnv {
			c.CacheRedis[0].Host = secretConfig.Cache.Redis.Address()
			c.CacheRedis[0].Pass = secretConfig.Cache.Redis.Password
		} else {
			// Local/dev：只覆盖密码（避免覆盖 host 默认值导致连错 Redis）
			if secretConfig.Cache.Redis.Password != "" {
				c.CacheRedis[0].Pass = secretConfig.Cache.Redis.Password
			}
		}
	}

	// ChainRPC upstream API keys (optional): load from SecretConfig (preferred in AWS).
	if secretConfig.ChainRPC.EthRpcApiKey != "" || secretConfig.ChainRPC.BscRpcApiKey != "" || secretConfig.ChainRPC.TronApiKey != "" {
		for i := range c.Chains {
			switch c.Chains[i].ChainType {
			case "ETH":
				if secretConfig.ChainRPC.EthRpcApiKey != "" {
					c.Chains[i].APIKey = secretConfig.ChainRPC.EthRpcApiKey
				}
			case "BSC":
				if secretConfig.ChainRPC.BscRpcApiKey != "" {
					c.Chains[i].APIKey = secretConfig.ChainRPC.BscRpcApiKey
				}
			case "TRON":
				if secretConfig.ChainRPC.TronApiKey != "" {
					c.Chains[i].TronAPIKey = secretConfig.ChainRPC.TronApiKey
				}
			}
		}
	}

	// 应用环境变量覆盖（其他非敏感配置）
	if err := config.ApplyEnvOverrides(&c); err != nil {
		logx.Errorf("Failed to apply env overrides: %v", err)
	}

	// 从环境变量读取 NodeID（优先级高于配置文件）
	c.NodeID = utils.LoadNodeIDFromEnv("CHAINRPC_NODE_ID", c.NodeID)
	logx.Infof("Starting ChainRPC RPC with NodeID: %d", c.NodeID)

	svcCtx, err := svc.NewServiceContext(c)
	if err != nil {
		log.Fatalf("Failed to create service context: %v", err)
	}

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		pb.RegisterChainRPCServer(grpcServer, server.NewChainRPCServer(svcCtx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	defer s.Stop()

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}
