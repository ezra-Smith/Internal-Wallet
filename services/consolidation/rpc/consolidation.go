package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"internalwallet/common/interceptor"
	"internalwallet/pkg/secrets"
	"internalwallet/proto/pb"
	"internalwallet/services/consolidation/rpc/internal/config"
	"internalwallet/services/consolidation/rpc/internal/repository"
	"internalwallet/services/consolidation/rpc/internal/scheduler"
	"internalwallet/services/consolidation/rpc/internal/server"
	"internalwallet/services/consolidation/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/consolidation.yaml", "the config file")

func main() {
	flag.Parse()

	// Step 1: load secrets (highest priority)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	secretConfig, err := secrets.LoadFromConfigMode(ctx, *configFile)
	if err != nil {
		logx.Errorf("Failed to load secrets: %v", err)
		panic(err)
	}
	logx.Infof("Secrets loaded successfully (environment: %s)", secretConfig.Environment)

	var c config.Config
	conf.MustLoad(*configFile, &c)

	// Step 2: inject sensitive configs (avoid overriding local host/port/database in dev)
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
		} else {
			if secretConfig.Cache.Redis.Password != "" {
				c.CacheRedis[0].Pass = secretConfig.Cache.Redis.Password
			}
		}
	}

	// Step 3: iTRX secrets for EnergyRental
	// Prefer SecretConfig (CSI mount) in AWS environments; allow env overrides for emergency/manual ops.
	if isAWSSecretsEnv {
		if v := strings.TrimSpace(secretConfig.Consolidation.ITRXApiEndpoint); v != "" {
			c.Consolidation.EnergyRental.ApiEndpoint = v
		}
		if v := strings.TrimSpace(secretConfig.Consolidation.ITRXApiKey); v != "" {
			c.Consolidation.EnergyRental.ApiKey = v
		}
		if v := strings.TrimSpace(secretConfig.Consolidation.ITRXApiSecret); v != "" {
			c.Consolidation.EnergyRental.ApiSecret = v
		}
	}

	// Step 3.1: chain upstream API keys (reuse ChainRPC secrets on the shared CSI mount)
	if isAWSSecretsEnv {
		if secretConfig.ChainRPC.EthRpcApiKey != "" || secretConfig.ChainRPC.BscRpcApiKey != "" || secretConfig.ChainRPC.TronApiKey != "" {
			for i := range c.Chains {
				switch strings.ToUpper(strings.TrimSpace(c.Chains[i].ChainType)) {
				case "ETH":
					if strings.TrimSpace(c.Chains[i].APIKey) == "" && secretConfig.ChainRPC.EthRpcApiKey != "" {
						c.Chains[i].APIKey = secretConfig.ChainRPC.EthRpcApiKey
					}
				case "BSC":
					if strings.TrimSpace(c.Chains[i].APIKey) == "" && secretConfig.ChainRPC.BscRpcApiKey != "" {
						c.Chains[i].APIKey = secretConfig.ChainRPC.BscRpcApiKey
					}
				case "TRON":
					if strings.TrimSpace(c.Chains[i].TronAPIKey) == "" && secretConfig.ChainRPC.TronApiKey != "" {
						c.Chains[i].TronAPIKey = secretConfig.ChainRPC.TronApiKey
					}
				}
			}
		}
	}

	if v := os.Getenv("CONSOLIDATION_ITRX_API_ENDPOINT"); v != "" {
		c.Consolidation.EnergyRental.ApiEndpoint = v
	}
	if v := os.Getenv("CONSOLIDATION_ITRX_API_KEY"); v != "" {
		c.Consolidation.EnergyRental.ApiKey = v
	}
	if v := os.Getenv("CONSOLIDATION_ITRX_API_SECRET"); v != "" {
		c.Consolidation.EnergyRental.ApiSecret = v
	}

	svcCtx := svc.NewServiceContext(c)
	if err := validateAndNormalizeConsolidationConfig(ctx, svcCtx); err != nil {
		logx.Severef("Consolidation config validation failed: %v", err)
		panic(err)
	}

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		pb.RegisterConsolidationServer(grpcServer, server.NewConsolidationServer(svcCtx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	// Receive metadata (user_id, client_ip, etc.) if callers provide it.
	s.AddUnaryInterceptors(interceptor.ServerMetadataUnaryInterceptor())

	// Background workers
	var wg sync.WaitGroup
	wg.Add(7)
	go func() {
		defer wg.Done()
		svc.RunRuntimeReloader(ctx, svcCtx, 2*time.Second)
	}()
	go func() {
		defer wg.Done()
		scheduler.NewConsolidationScheduler(svcCtx).Run(ctx)
	}()
	go func() {
		defer wg.Done()
		scheduler.NewExecutor(svcCtx).Run(ctx)
	}()
	go func() {
		defer wg.Done()
		scheduler.NewEnergyRentalWorker(svcCtx).Run(ctx)
	}()
	go func() {
		defer wg.Done()
		scheduler.NewStatusTracker(svcCtx).Run(ctx)
	}()
	go func() {
		defer wg.Done()
		scheduler.NewTopUpWorker(svcCtx).Run(ctx)
	}()
	go func() {
		defer wg.Done()
		scheduler.NewTopUpVerifyWorker(svcCtx).Run(ctx)
	}()

	// Graceful shutdown: cancel workers and stop server on SIGINT/SIGTERM.
	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stopCh
		logx.Info("shutdown signal received; stopping consolidation service...")
		cancel()
		s.Stop()
	}()

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
	wg.Wait()
}

func validateAndNormalizeConsolidationConfig(ctx context.Context, svcCtx *svc.ServiceContext) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if svcCtx == nil {
		return fmt.Errorf("svcCtx is nil")
	}
	cfg := svcCtx.Config.Consolidation
	if !cfg.Enabled {
		return nil
	}

	// Fail-fast: consolidation service must not run with missing critical dependencies/config.
	if svcCtx.DB == nil {
		return fmt.Errorf("MySQL is not configured/connected")
	}
	if svcCtx.Chain == nil {
		return fmt.Errorf("chain nodes are not configured/connected")
	}
	if svcCtx.SignerRpc == nil {
		return fmt.Errorf("SignerRpc is not configured/connected")
	}

	// Fail-fast: schema must match the assumptions made by multi-instance leasing and tx tracking.
	if err := repository.EnsureConsolidationSchema(svcCtx.DB); err != nil {
		return err
	}
	if cfg.EnergyRental.Enabled {
		if strings.TrimSpace(cfg.EnergyRental.ApiEndpoint) == "" || strings.TrimSpace(cfg.EnergyRental.ApiKey) == "" || strings.TrimSpace(cfg.EnergyRental.ApiSecret) == "" {
			return fmt.Errorf("EnergyRental.Enabled=true but iTRX api endpoint/key/secret is missing")
		}
		if cfg.EnergyRental.MaxOrdersPerTask <= 0 {
			return fmt.Errorf("EnergyRental.MaxOrdersPerTask must be > 0")
		}
		if cfg.EnergyRental.MaxEnergyPerOrder <= 0 {
			return fmt.Errorf("EnergyRental.MaxEnergyPerOrder must be > 0")
		}
		if cfg.EnergyRental.MaxTotalCostSunPerTask <= 0 {
			return fmt.Errorf("EnergyRental.MaxTotalCostSunPerTask must be > 0")
		}
	}
	if cfg.EvmGasBumpMultiplier <= 1.0 {
		return fmt.Errorf("EvmGasBumpMultiplier must be > 1.0")
	}
	if cfg.EvmBumpAfterSeconds < 0 {
		return fmt.Errorf("EvmBumpAfterSeconds must be >= 0")
	}
	if cfg.EvmMaxBumps < 0 {
		return fmt.Errorf("EvmMaxBumps must be >= 0")
	}

	// Normalize config maps to avoid case-related misconfigurations.
	cfg.TargetAddresses = normalizeNestedStringMap(cfg.TargetAddresses)
	cfg.MinBalanceThresholds = normalizeNestedStringMap(cfg.MinBalanceThresholds)
	cfg.NativeReserves = normalizeNestedStringMap(cfg.NativeReserves)
	cfg.MinConsolidationAmount = normalizeNestedStringMap(cfg.MinConsolidationAmount)
	cfg.MaxGasPrice = normalizeStringMap(cfg.MaxGasPrice)
	cfg.Risk.MaxNativeSpendPerTask = normalizeStringMap(cfg.Risk.MaxNativeSpendPerTask)
	cfg.GasSafetyMultipliers = normalizeFloat64Map(cfg.GasSafetyMultipliers)

	// Validate FeeGuard config early (USDT-only, EVM-only).
	if cfg.FeeGuard.Enabled {
		if cfg.FeeGuard.CheckIntervalSeconds <= 0 {
			return fmt.Errorf("FeeGuard.CheckIntervalSeconds must be > 0 when FeeGuard.Enabled=true")
		}
		if cfg.FeeGuard.BlockSeconds <= 0 {
			return fmt.Errorf("FeeGuard.BlockSeconds must be > 0 when FeeGuard.Enabled=true")
		}
		if cfg.FeeGuard.EvmUsdtTransferGasLimit == 0 {
			return fmt.Errorf("FeeGuard.EvmUsdtTransferGasLimit must be > 0 when FeeGuard.Enabled=true")
		}
	}

	// Normalize and validate ToAddressMode.
	toMode := strings.ToLower(strings.TrimSpace(cfg.ToAddressMode))
	if toMode == "" {
		toMode = "target_addresses"
	}
	switch toMode {
	case "target_addresses", "system_hot_wallet":
	default:
		return fmt.Errorf("unsupported ToAddressMode: %q (allowed: target_addresses, system_hot_wallet)", cfg.ToAddressMode)
	}
	cfg.ToAddressMode = toMode

	if toMode == "system_hot_wallet" {
		if strings.TrimSpace(cfg.SystemHotWalletAddressType) == "" {
			return fmt.Errorf("SystemHotWalletAddressType is required when ToAddressMode=system_hot_wallet")
		}
		if cfg.SystemHotWalletTemperature < 0 {
			return fmt.Errorf("SystemHotWalletTemperature must be >= 0")
		}
	}

	// Normalize and validate TRON TRC20 fee mode.
	tronTrc20Mode := strings.ToLower(strings.TrimSpace(cfg.TronTrc20FeeMode))
	if tronTrc20Mode == "" {
		tronTrc20Mode = "energy_rental"
	}
	switch tronTrc20Mode {
	case "energy_rental", "trx_fee":
	default:
		return fmt.Errorf("unsupported TronTrc20FeeMode: %q (allowed: energy_rental, trx_fee)", cfg.TronTrc20FeeMode)
	}
	cfg.TronTrc20FeeMode = tronTrc20Mode

	// Validate TopUp config early (fail-fast to avoid accidental infinite retries/spend).
	if cfg.TopUp.Enabled {
		if strings.TrimSpace(cfg.TopUp.HotWalletAddressType) == "" {
			return fmt.Errorf("TopUp.HotWalletAddressType is required when TopUp.Enabled=true")
		}
		if cfg.TopUp.HotWalletTemperature < 0 {
			return fmt.Errorf("TopUp.HotWalletTemperature must be >= 0")
		}
		if cfg.TopUp.MaxTopUpAttemptsPerTask <= 0 {
			return fmt.Errorf("TopUp.MaxTopUpAttemptsPerTask must be > 0")
		}
		if cfg.TopUp.TopUpInterval <= 0 {
			return fmt.Errorf("TopUp.TopUpInterval must be > 0")
		}
		if cfg.TopUp.VerifyInterval <= 0 {
			return fmt.Errorf("TopUp.VerifyInterval must be > 0")
		}
		if cfg.TopUp.TaskRetryDelaySeconds <= 0 {
			return fmt.Errorf("TopUp.TaskRetryDelaySeconds must be > 0")
		}
	}

	svcCtx.Config.Consolidation = cfg

	// Validate strategy config early (avoid runtime scheduler errors).
	switch strings.ToLower(strings.TrimSpace(cfg.Strategy)) {
	case "", "immediate":
	case "scheduled":
		if _, err := time.Parse("15:04", strings.TrimSpace(cfg.ScheduledTime)); err != nil {
			return fmt.Errorf("invalid ScheduledTime %q: %w", cfg.ScheduledTime, err)
		}
	default:
		return fmt.Errorf("unsupported Strategy: %s", cfg.Strategy)
	}

	// Initial runtime load (best-effort): service can start in idle mode and retry in background.
	if err := svcCtx.ReloadRuntimeFromDB(ctx); err != nil {
		logx.WithContext(ctx).Errorf("initial consolidation runtime reload failed (service will run in idle and retry): %v", err)
	}

	return nil
}

func normalizeStringMap(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		nk := strings.ToUpper(strings.TrimSpace(k))
		if nk == "" {
			continue
		}
		out[nk] = strings.TrimSpace(v)
	}
	return out
}

func normalizeNestedStringMap(m map[string]map[string]string) map[string]map[string]string {
	out := make(map[string]map[string]string, len(m))
	for chain, inner := range m {
		c := strings.ToUpper(strings.TrimSpace(chain))
		if c == "" {
			continue
		}
		if out[c] == nil {
			out[c] = make(map[string]string, len(inner))
		}
		for asset, val := range inner {
			a := strings.ToUpper(strings.TrimSpace(asset))
			if a == "" {
				continue
			}
			out[c][a] = strings.TrimSpace(val)
		}
	}
	return out
}

func normalizeFloat64Map(m map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(m))
	for k, v := range m {
		nk := strings.ToUpper(strings.TrimSpace(k))
		if nk == "" {
			continue
		}
		out[nk] = v
	}
	return out
}
