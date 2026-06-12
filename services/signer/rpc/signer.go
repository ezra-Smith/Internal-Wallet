package main

import (
	"context"
	"flag"
	"fmt"

	"internalwallet/common/interceptor"
	"internalwallet/pkg/secrets"
	"internalwallet/proto/pb"
	"internalwallet/services/signer/rpc/internal/config"
	"internalwallet/services/signer/rpc/internal/server"
	"internalwallet/services/signer/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/signer.yaml", "the config file")

func main() {
	flag.Parse()

	// Step 1: 加载密钥配置（优先级最高）
	ctx := context.Background()

	// 从配置文件 Mode 自动读取环境
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
	// IMPORTANT:
	// - 在本地/开发（secrets.Environment=local 等）场景下，secrets loader 会为 Host/Port/Database 提供默认值
	//   若这里无条件覆盖，会把配置文件中正确的 SIGNER_MYSQL_* 覆盖成 localhost 默认值，导致 DB 连接失败。
	// - 因此：仅在从 AWS Secrets（production）加载时，才“完整注入” MySQL 的 host/port/database；
	//   在本地环境只覆盖真正的敏感项（password 等），且仅当 secrets 中有值时才覆盖。
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
			// Local/dev：只覆盖密码（避免覆盖 host 默认值导致连错 Redis）
			if secretConfig.Cache.Redis.Password != "" {
				c.CacheRedis[0].Pass = secretConfig.Cache.Redis.Password
			}
		}
	}

	// Signer 加密密码（最关键的密钥）
	// Local/dev：若 secrets 中为空，不覆盖（避免把配置文件中的 SIGNER_ENCRYPTION_PASSWORD 覆盖成空）
	if isAWSSecretsEnv || secretConfig.Security.Signer.EncryptionPassword != "" {
		c.Security.EncryptionPassword = secretConfig.Security.Signer.EncryptionPassword
	}

	// Step 4: 验证关键配置（防止使用弱密码）
	// 注意：热钱包主流程已改为“管理员解锁密码”，因此 signer 本身不应因 EncryptionPassword 未配置而无法启动。
	// 但系统内仍有其他逻辑可能使用该配置（例如用户助记词备份挑战等），因此这里只做告警，不做 panic。
	if len(c.Security.EncryptionPassword) < 32 {
		logx.Errorf("Signer encryption password is weak or not configured (< 32 chars); service will start but some features may be unavailable")
	} else {
		logx.Infof("Signer encryption password validated (length: %d)", len(c.Security.EncryptionPassword))
	}

	// Step 5: 创建服务上下文（原有逻辑）
	svcCtx := svc.NewServiceContext(c)

	// 初始化IP白名单拦截器
	ipWhitelistInterceptor, err := interceptor.NewIPWhitelistInterceptor(
		&interceptor.IPWhitelistConfig{
			Enabled:      c.Security.EnableIPWhitelist,
			AllowedIPs:   c.Security.AllowedIPs,
			AllowedCIDRs: c.Security.AllowedCIDRs,
		},
		"signer.rpc",
	)
	if err != nil {
		logx.Errorf("Failed to initialize IP whitelist interceptor: %v", err)
		panic(err)
	}

	// 创建RPC服务器，注册拦截器
	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		pb.RegisterSignerServiceServer(grpcServer, server.NewSignerServiceServer(svcCtx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})

	// 添加拦截器（按执行顺序：先解析元数据，再检查IP白名单）
	s.AddUnaryInterceptors(
		interceptor.ServerMetadataUnaryInterceptor(),    // 1. 解析元数据（user_id、client_ip等）
		ipWhitelistInterceptor.UnaryServerInterceptor(), // 2. IP白名单检查
	)

	defer s.Stop()

	fmt.Printf("Starting signer rpc server at %s...\n", c.ListenOn)
	if c.Security.EnableIPWhitelist {
		logx.Infof("IP whitelist enabled - Allowed IPs: %v, CIDRs: %v",
			ipWhitelistInterceptor.GetAllowedIPs(),
			ipWhitelistInterceptor.GetAllowedCIDRs())
	} else {
		logx.Infof("IP whitelist disabled - All IPs allowed (not recommended for production)")
	}

	s.Start()
}
