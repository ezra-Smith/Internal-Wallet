package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	commoninterceptor "internalwallet/common/interceptor"
	"internalwallet/pkg/secrets"
	"internalwallet/proto/pb"
	"internalwallet/services/swap/rpc/internal/config"
	swapinterceptor "internalwallet/services/swap/rpc/internal/interceptor"
	"internalwallet/services/swap/rpc/internal/server"
	"internalwallet/services/swap/rpc/internal/svc"
)

var configFile = flag.String("f", "etc/swap.yaml", "the config file")

func main() {
	flag.Parse()

	// Step 1: Load secrets (CSI mount in k8s; .env for local)
	ctx := context.Background()
	secretConfig, err := secrets.LoadFromConfigMode(ctx, *configFile)
	if err != nil {
		logx.Errorf("Failed to load secrets: %v", err)
		panic(err)
	}
	logx.Infof("Secrets loaded successfully (environment: %s)", secretConfig.Environment)

	var c config.Config
	conf.MustLoad(*configFile, &c)

	// Step 2: Inject base infra secrets (DB/Redis)
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

	if len(c.CacheRedis) > 0 {
		if isAWSSecretsEnv {
			c.CacheRedis[0].Host = secretConfig.Cache.Redis.Address()
			c.CacheRedis[0].Pass = secretConfig.Cache.Redis.Password
		} else if secretConfig.Cache.Redis.Password != "" {
			c.CacheRedis[0].Pass = secretConfig.Cache.Redis.Password
		}
	}

	// Step 3: Inject SWAP-specific secrets from SecretConfig (for AWS environments)
	if isAWSSecretsEnv {
		config.ApplySecretsConfig(&c, secretConfig)
	}

	// Step 4: Allow env var overrides for backward compatibility / local dev
	config.ApplyEnvOverrides(&c)
	svcCtx := svc.NewServiceContext(c)

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		pb.RegisterSwapServer(grpcServer, server.NewSwapServer(svcCtx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	s.AddUnaryInterceptors(
		commoninterceptor.ServerMetadataUnaryInterceptor(),
		//测试代码暂时先注释
		swapinterceptor.APIKeyAuthUnaryInterceptor(svcCtx),
	)
	defer s.Stop()

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}
