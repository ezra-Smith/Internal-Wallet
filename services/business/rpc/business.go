package main

import (
	"context"
	"flag"
	"fmt"
	"internalwallet/common/interceptor"
	"internalwallet/pkg/notify"
	"internalwallet/pkg/secrets"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/config"
	"internalwallet/services/business/rpc/internal/consumer"
	"internalwallet/services/business/rpc/internal/logic"
	"internalwallet/services/business/rpc/internal/server"
	"internalwallet/services/business/rpc/internal/svc"
	"strings"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/business.yaml", "the config file")

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
	// IMPORTANT:
	// - 在本地/开发（secrets.Environment=local 等）场景下，secrets loader 会为 Host/Port/Database 提供默认值
	//   若这里无条件覆盖，会把配置文件中正确的 BUSINESS_MYSQL_* 覆盖成 localhost 默认值，导致 DB 连接失败。
	// - 因此：仅在从 AWS Secrets（production）加载时，才“完整注入” MySQL 的 host/port/database；
	//   在本地环境只覆盖真正的敏感项（password/jwt 等），且仅当 secrets 中有值时才覆盖。
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

	// JWT 配置
	// Local/dev：若 secrets 中为空，不覆盖（避免把配置文件中的 BUSINESS_JWT_* 覆盖成空）
	if isAWSSecretsEnv || secretConfig.Security.JWT.AccessSecret != "" {
		c.JWT.AccessSecret = secretConfig.Security.JWT.AccessSecret
	}
	if isAWSSecretsEnv || secretConfig.Security.JWT.RefreshSecret != "" {
		c.JWT.RefreshSecret = secretConfig.Security.JWT.RefreshSecret
	}

	// 邮箱配置：从 secrets 加载（本地环境从 .env 加载）
	if secretConfig.Notifications.SMTP.Host != "" {
		c.Email.SMTPHost = secretConfig.Notifications.SMTP.Host
	}
	if secretConfig.Notifications.SMTP.Port > 0 {
		c.Email.SMTPPort = fmt.Sprintf("%d", secretConfig.Notifications.SMTP.Port)
	}
	if secretConfig.Notifications.SMTP.FromAddress != "" {
		c.Email.FromAddress = secretConfig.Notifications.SMTP.FromAddress
	}
	if secretConfig.Notifications.SMTP.Password != "" {
		// Gmail 应用专用密码可能包含空格，需要去掉
		c.Email.FromPassword = strings.ReplaceAll(strings.TrimSpace(secretConfig.Notifications.SMTP.Password), " ", "")
	}

	// Step 3.5: 注入 Business-specific secrets from SecretConfig (for AWS environments)
	if isAWSSecretsEnv {
		config.ApplySecretsConfig(&c, secretConfig)
	}

	// Step 3.6: Initialize SMS from secrets (for AWS environments)
	notify.InitFromSecretConfig(secretConfig)

	// 应用环境变量覆盖（其他非敏感配置）
	config.ApplyEnvOverrides(&c)

	// ========================================
	// Step 4: 创建服务上下文（原有逻辑）
	// ========================================
	svcCtx := svc.NewServiceContext(c)

	// Background worker: payout + on-chain confirmation + settlement for withdrawals.
	go logic.NewWithdrawPayoutWorker(svcCtx).Run(context.Background())
	// Background worker: best-effort refresh for pending Web3 broadcast placeholders.
	go logic.NewWeb3PlaceholderReconcileWorker(svcCtx, c.Web3PlaceholderReconcile).Run(context.Background())

	// 初始化并启动 Kafka 消费者（如果已配置）
	// 注意：在这里初始化避免循环依赖（consumer 包导入 svc，svc 不能导入 consumer）
	if len(c.KafkaConsumer.Brokers) > 0 && c.KafkaConsumer.GroupID != "" {
		kafkaConsumer, err := consumer.NewKafkaConsumer(svcCtx, c)
		if err != nil {
			logx.Errorf("Failed to create Kafka consumer: %v (will continue without it)", err)
		} else {
			svcCtx.KafkaConsumer = kafkaConsumer
			logx.Info("✓ Kafka consumer initialized successfully")

			// 启动 Kafka 消费者
			if err := kafkaConsumer.Start(); err != nil {
				logx.Errorf("Failed to start Kafka consumer: %v", err)
			} else {
				defer func() {
					if err := kafkaConsumer.Stop(); err != nil {
						logx.Errorf("Failed to stop Kafka consumer: %v", err)
					}
				}()
			}
		}
	} else {
		logx.Info("KafkaConsumer config not found, message consumption will be disabled")
	}

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		pb.RegisterBusinessServer(grpcServer, server.NewBusinessServer(svcCtx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})

	// 添加服务端元数据拦截器（接收 user_id、email 等信息）
	s.AddUnaryInterceptors(interceptor.ServerMetadataUnaryInterceptor())

	defer s.Stop()

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}
