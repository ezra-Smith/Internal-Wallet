package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"internalwallet/pkg/secrets"
	"internalwallet/services/market/rpc/internal/binance"
	"internalwallet/services/market/rpc/internal/config"
	"internalwallet/services/market/rpc/internal/fiat"
	"internalwallet/services/market/rpc/internal/health"
	"internalwallet/services/market/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
)

var configFile = flag.String("f", "etc/market.yaml", "the config file")

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

	// Step 3: 注入密钥到配置（在 ApplyEnvOverrides 之前）
	if len(c.CacheRedis) > 0 {
		c.CacheRedis[0].Host = secretConfig.Cache.Redis.Address()
		c.CacheRedis[0].Pass = secretConfig.Cache.Redis.Password
	}

	if err := config.ApplyEnvOverrides(&c); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to apply env overrides: %v\n", err)
		os.Exit(1)
	}

	// In offline mode (Enabled=false), health should not require WS freshness.
	if !c.Enabled {
		c.Health.RequireWS = false
	}

	c.MustSetUp()

	if err := c.Validate(); err != nil {
		logx.Severef("Invalid config: %v", err)
	}

	logx.Infof("Redis ticker hash key: %s (ttl=%ds)", c.Redis.TickerHashKey, c.Redis.TTLSeconds)
	if c.Enabled {
		wsURL, err := c.WSURL()
		if err != nil {
			logx.Severef("Invalid Binance WS URL config: %v", err)
		}
		logx.Infof("Binance WS: %s", wsURL)
	} else {
		logx.Infof("Market external collectors disabled (Enabled=false). Running in health-only mode.")
	}

	svcCtx := svc.NewServiceContext(c)
	defer func() {
		_ = svcCtx.RedisClient.Close()
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var healthServer *health.Server
	if c.Health.Enabled {
		healthServer = health.NewServer(c.Health, svcCtx.RedisClient, svcCtx.Status)
		go func() {
			if err := healthServer.Start(); err != nil {
				logx.Errorf("Health server stopped: %v", err)
			}
		}()
		defer func() {
			_ = healthServer.Shutdown(context.Background())
		}()
	}

	if !c.Enabled {
		<-ctx.Done()
		return
	}

	// Create sparkline collector (samples prices periodically for mini charts)
	// Uses Redis ZSET for efficient time-range queries
	var sparklineCollector *binance.SparklineCollector
	if c.Sparkline.Enabled {
		sparklineCollector = binance.NewSparklineCollector(c, svcCtx.RedisClient)
		go sparklineCollector.Run(ctx)
		logx.Infof("Sparkline collector enabled: interval=%s, retention=%s",
			c.SparklineSampleInterval(), c.SparklineRetention())
	}

	// Create fiat collector (periodically fetches USDT->fiat mid rates via Binance C2C)
	if c.Fiat.Enabled {
		fiatCollector := fiat.NewCollector(c, svcCtx.RedisClient)
		go fiatCollector.Run(ctx)
		logx.Infof("Fiat collector enabled: interval=%s, ttl=%s, hashKey=%s",
			c.FiatUpdateInterval(), c.FiatRedisTTL(), c.Fiat.RedisHashKey)
	}

	streamer := binance.NewMiniTickerStreamer(c, svcCtx.RedisClient, svcCtx.Status, sparklineCollector)
	if err := streamer.Run(ctx); err != nil {
		logx.Errorf("Streamer exited: %v", err)
	}
}
