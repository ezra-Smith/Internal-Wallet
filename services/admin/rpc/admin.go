package main

import (
	"context"
	"flag"
	"fmt"
	"internalwallet/pkg/secrets"
	"strings"
	"time"

	commoninterceptor "internalwallet/common/interceptor"
	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/config"
	admininterceptor "internalwallet/services/admin/rpc/internal/interceptor"
	"internalwallet/services/admin/rpc/internal/logic"
	"internalwallet/services/admin/rpc/internal/server"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/admin.yaml", "the config file")

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

	// Step 2: Inject secrets into config (overwrite YAML placeholders)
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

	// Admin JWT
	if isAWSSecretsEnv || secretConfig.Security.AdminJWT.AccessSecret != "" {
		c.JWT.AccessSecret = secretConfig.Security.AdminJWT.AccessSecret
	}
	if isAWSSecretsEnv || secretConfig.Security.AdminJWT.RefreshSecret != "" {
		c.JWT.RefreshSecret = secretConfig.Security.AdminJWT.RefreshSecret
	}

	// SMTP (Admin emails): inject from SecretConfig when present.
	// Keep YAML fields as placeholders; do not commit sensitive values.
	if v := strings.TrimSpace(secretConfig.Notifications.SMTP.Host); v != "" {
		c.Email.SMTPHost = v
	}
	if secretConfig.Notifications.SMTP.Port > 0 {
		c.Email.SMTPPort = fmt.Sprintf("%d", secretConfig.Notifications.SMTP.Port)
	}
	if v := strings.TrimSpace(secretConfig.Notifications.SMTP.FromAddress); v != "" {
		c.Email.FromAddress = v
	}
	if v := strings.TrimSpace(secretConfig.Notifications.SMTP.FromName); v != "" {
		c.Email.FromName = v
	}
	if v := strings.TrimSpace(secretConfig.Notifications.SMTP.Password); v != "" {
		// Some SMTP app passwords may contain spaces.
		c.Email.FromPassword = strings.ReplaceAll(v, " ", "")
	}

	// Step 3: Inject Admin-specific secrets from SecretConfig (for AWS environments)
	if isAWSSecretsEnv {
		config.ApplySecretsConfig(&c, secretConfig)
	}

	// Step 4: Allow env var overrides for backward compatibility / local dev
	config.ApplyEnvOverrides(&c)

	svcCtx := svc.NewServiceContext(c)

	// Background worker: transfer batch execution (internal ledger credits).
	if c.TransferBatchWorker.Enabled {
		go logic.NewTransferBatchWorker(svcCtx, logic.TransferBatchWorkerOptions{
			PollInterval:     time.Duration(c.TransferBatchWorker.PollIntervalSeconds) * time.Second,
			BatchSize:        c.TransferBatchWorker.BatchSize,
			ItemTimeout:      time.Duration(c.TransferBatchWorker.ItemTimeoutSeconds) * time.Second,
			ProcessingLease:  time.Duration(c.TransferBatchWorker.ProcessingLeaseSeconds) * time.Second,
			MinRetryInterval: time.Duration(c.TransferBatchWorker.MinRetryIntervalSeconds) * time.Second,
		}).Run(context.Background())
	}

	// Background worker: vault balance sync scheduler (定期同步金库地址余额)
	if c.VaultBalanceSyncScheduler.Enabled {
		go logic.NewVaultBalanceSyncScheduler(svcCtx, time.Duration(c.VaultBalanceSyncScheduler.SyncIntervalSeconds)*time.Second).Run(context.Background())
	}

	// Background worker: vault balance telegram notification (金库余额Telegram通知)
	if c.VaultBalanceNotification.Enabled {
		logx.Infof("启动金库余额Telegram通知调度器...")
		logx.Infof("配置: Enabled=%v, NotificationIntervalSeconds=%d",
			c.VaultBalanceNotification.Enabled,
			c.VaultBalanceNotification.NotificationIntervalSeconds)
		go logic.NewVaultBalanceNotificationScheduler(svcCtx, time.Duration(c.VaultBalanceNotification.NotificationIntervalSeconds)*time.Second).Run(context.Background())
	} else {
		logx.Info("⚠️  金库余额Telegram通知调度器未启用 (VaultBalanceNotification.Enabled=false)")
	}

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		pb.RegisterAdminServer(grpcServer, server.NewAdminServer(svcCtx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	// 元数据透传 + Admin 鉴权/RBAC/幂等
	s.AddUnaryInterceptors(
		commoninterceptor.ServerMetadataUnaryInterceptor(),
		admininterceptor.AuditLogUnaryInterceptor(svcCtx),
		admininterceptor.AdminAuthUnaryInterceptor(svcCtx),
	)
	defer s.Stop()

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}
