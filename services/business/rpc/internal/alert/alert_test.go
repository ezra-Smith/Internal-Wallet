package alert

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/zrpc"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"internalwallet/common/notification"
	"internalwallet/proto/pb"
	businessConfig "internalwallet/services/business/rpc/internal/config"
	"internalwallet/services/business/rpc/internal/svc"
)

// initAdminRpcClient 初始化 AdminRpc 客户端
func initAdminRpcClient(c businessConfig.Config) pb.AdminClient {
	if len(c.AdminRpc.Etcd.Hosts) == 0 && len(c.AdminRpc.Endpoints) == 0 {
		logx.Infof("AdminRpc config not found in business.yaml")
		return nil
	}

	logx.Infof("Connecting to AdminRpc: etcd_hosts=%v, endpoints=%v",
		c.AdminRpc.Etcd.Hosts, c.AdminRpc.Endpoints)

	adminConn, err := zrpc.NewClient(c.AdminRpc)
	if err != nil {
		logx.Errorf("Failed to connect to AdminRpc: %v", err)
		return nil
	}

	logx.Info("AdminRpc client initialized successfully")
	return pb.NewAdminClient(adminConn.Conn())
}

// TestAlertFlow 测试预警流程
func TestAlertFlow(t *testing.T) {
	// 1. 加载配置
	var c businessConfig.Config
	conf.MustLoad("../../etc/business.yaml", &c)

	t.Logf("配置加载成功，MySQL: %s@%s/%s", c.MySQL.Username, c.MySQL.Host, c.MySQL.Database)

	// 2. 初始化数据库连接
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		c.MySQL.Username, c.MySQL.Password, c.MySQL.Host, c.MySQL.Database)
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}

	// 测试数据库连接
	sqlDB, _ := db.DB()
	if sqlDB != nil {
		if err := sqlDB.Ping(); err != nil {
			t.Logf("Warning: Database ping failed: %v", err)
		} else {
			t.Log("Database connection successful")
		}
	}

	// 3. 初始化 Redis 客户端
	if len(c.CacheRedis) == 0 {
		t.Fatal("CacheRedis configuration is empty, please check business.yaml")
	}

	t.Logf("Redis配置: Host=%s", c.CacheRedis[0].Host)

	redisClient := redis.NewClient(&redis.Options{
		Addr:         c.CacheRedis[0].Host,
		Password:     c.CacheRedis[0].Pass,
		DB:           0,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	// 测试 Redis 连接
	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		t.Fatalf("Redis connection failed: %v", err)
	}
	t.Log("Redis connection successful")

	// 4. 初始化 AdminRpc 客户端
	adminRpcClient := initAdminRpcClient(c)
	if adminRpcClient == nil {
		t.Log("Warning: AdminRpc not available, config will be loaded from Redis cache only")
	}

	// 5. 初始化 ServiceContext
	svcCtx := &svc.ServiceContext{
		DB:          db,
		RedisClient: redisClient,
		AdminRpc:    adminRpcClient,
	}

	// 6. 初始化 AlertChecker
	alertChecker := NewAlertChecker(svcCtx)

	// 7. 清除旧的 Redis 缓存，让配置重新加载
	redisClient.Del(ctx, "alert:config:all")
	redisClient.Del(ctx, "system:config:telegram:cached")
	t.Log("Cleared old Redis caches")

	// 8. 等待一下让配置生效
	time.Sleep(1 * time.Second)

	// 9. 模拟 3 次提现，每次间隔 20 秒，总共 1 分钟内完成
	testWithdrawals := []struct {
		assetCode string
		amount    string
		delay     time.Duration
	}{
		{"USDT", "30000", 0 * time.Second},  // 第1次：30000 USDT
		{"USDT", "30000", 5 * time.Second},  // 第2次：30000 USDT
		{"USDT", "40000", 10 * time.Second}, // 第3次：40000 USDT
	}

	totalUSD := decimal.Zero

	for i, withdrawal := range testWithdrawals {
		if i > 0 {
			t.Logf("等待 %v 后发送第 %d 次提现...", withdrawal.delay, i+1)
			time.Sleep(withdrawal.delay)
		}

		amount, _ := decimal.NewFromString(withdrawal.amount)
		amountUSD := amount // USDT 直接等于 USD

		totalUSD = totalUSD.Add(amountUSD)

		t.Logf("[第%d次提现] asset=%s, amount=%s, 累计USD=%s",
			i+1, withdrawal.assetCode, amount.String(), totalUSD.String())

		// 调用预警检查
		err := alertChecker.CheckAndNotify(
			ctx,
			"web3_withdraw",
			withdrawal.assetCode,
			amount,
			amountUSD,
			map[string]interface{}{
				"order_id":   int64(8000000000 + int64(i)),
				"user_id":    int64(1001),
				"chain_code": "TRC20",
				"to_address": "TXyzTestAddress123456789",
			},
		)

		if err != nil {
			t.Errorf("第 %d 次预警检查失败: %v", i+1, err)
		} else {
			t.Logf("第 %d 次预警检查成功", i+1)
		}

		// 打印当前 Redis 中的累计数据
		printAccumulatedAmount(ctx, redisClient, 8000000001)
	}

	// 10. 等待异步通知完成
	t.Log("等待 5 秒让异步通知完成...")
	time.Sleep(5 * time.Second)

	t.Log("测试完成！请检查 Telegram 是否收到预警消息")
}

// TestAlertSingleConfig 测试单个配置的预警
func TestAlertSingleConfig(t *testing.T) {
	// 手动设置测试参数
	configID := int64(8000000001) // 测试配置 ID
	thresholdUSD := decimal.NewFromInt(100000)

	// 模拟单次大额提现
	testAmount := decimal.NewFromInt(120000) // 超过阈值

	t.Logf("测试：单次提现 %s USD，阈值 %s USD", testAmount.String(), thresholdUSD.String())

	// 加载配置
	var c businessConfig.Config
	conf.MustLoad("../../etc/business.yaml", &c)
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		c.MySQL.Username, c.MySQL.Password, c.MySQL.Host, c.MySQL.Database)
	db, _ := gorm.Open(mysql.Open(dsn), &gorm.Config{})

	// 初始化 Redis 客户端
	if len(c.CacheRedis) == 0 {
		t.Fatalf("CacheRedis configuration is empty")
	}
	redisClient := redis.NewClient(&redis.Options{
		Addr:         c.CacheRedis[0].Host,
		Password:     c.CacheRedis[0].Pass,
		DB:           0,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	// 初始化 AdminRpc
	adminRpcClient := initAdminRpcClient(c)

	svcCtx := &svc.ServiceContext{
		DB:          db,
		RedisClient: redisClient,
		AdminRpc:    adminRpcClient,
	}

	alertChecker := NewAlertChecker(svcCtx)

	ctx := context.Background()
	err := alertChecker.CheckAndNotify(
		ctx,
		"web3_withdraw",
		"USDT",
		testAmount,
		testAmount,
		map[string]interface{}{
			"order_id":   8000000001,
			"user_id":    1001,
			"chain_code": "TRC20",
		},
	)

	if err != nil {
		t.Errorf("预警检查失败: %v", err)
	}

	// 检查是否触发
	printAccumulatedAmount(ctx, redisClient, configID)

	t.Log("等待 3 秒...")
	time.Sleep(3 * time.Second)
	t.Log("测试完成！")
}

// TestTelegramConnection 测试 Telegram 连接
func TestTelegramConnection(t *testing.T) {
	var c businessConfig.Config
	conf.MustLoad("../../etc/business.yaml", &c)
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		c.MySQL.Username, c.MySQL.Password, c.MySQL.Host, c.MySQL.Database)
	db, _ := gorm.Open(mysql.Open(dsn), &gorm.Config{})

	// 初始化 Redis 客户端
	if len(c.CacheRedis) == 0 {
		t.Fatalf("CacheRedis configuration is empty")
	}
	redisClient := redis.NewClient(&redis.Options{
		Addr:         c.CacheRedis[0].Host,
		Password:     c.CacheRedis[0].Pass,
		DB:           0,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	// 初始化 AdminRpc
	adminRpcClient := initAdminRpcClient(c)

	svcCtx := &svc.ServiceContext{
		DB:          db,
		RedisClient: redisClient,
		AdminRpc:    adminRpcClient,
	}

	alertChecker := NewAlertChecker(svcCtx)

	ctx := context.Background()
	telegramCfg, err := alertChecker.getTelegramConfig(ctx)
	if err != nil {
		t.Fatalf("获取 Telegram 配置失败: %v", err)
	}

	t.Logf("Telegram 配置:")
	t.Logf("  BotToken: %s", maskString(telegramCfg.BotToken, 10, "..."))
	t.Logf("  ChatID: %s", telegramCfg.ChatID)
	t.Logf("  Enabled: %v", telegramCfg.Enabled)
	t.Logf("  TimeoutSeconds: %d", telegramCfg.TimeoutSeconds)

	if !telegramCfg.Enabled {
		t.Log("Telegram 未启用，请检查 admin_system_configs 表中的 telegram.enabled 配置")
	}

	if telegramCfg.BotToken == "" || telegramCfg.ChatID == "" {
		t.Log("Telegram BotToken 或 ChatID 未配置，请检查 admin_system_configs 表")
	}

	// 尝试发送测试消息
	t.Log("\n发送测试消息到 Telegram...")
	client := notification.NewTelegramClient(
		telegramCfg.BotToken,
		telegramCfg.ChatID,
		time.Duration(telegramCfg.TimeoutSeconds)*time.Second,
	)

	testMsg := "预警系统测试消息\n\n这是一条来自 Zink Wallet 预警系统的测试消息。如果您看到此消息，说明 Telegram 配置正确！"
	if err := client.SendMessage(testMsg); err != nil {
		t.Errorf("发送测试消息失败: %v", err)
	} else {
		t.Log("测试消息发送成功！请检查 Telegram 群组")
	}
}

// TestManualAlertCheck 手动检查当前预警状态
func TestManualAlertCheck(t *testing.T) {
	var c businessConfig.Config
	conf.MustLoad("../../etc/business.yaml", &c)
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		c.MySQL.Username, c.MySQL.Password, c.MySQL.Host, c.MySQL.Database)
	db, _ := gorm.Open(mysql.Open(dsn), &gorm.Config{})

	// 初始化 Redis 客户端
	if len(c.CacheRedis) == 0 {
		t.Fatalf("CacheRedis configuration is empty")
	}
	redisClient := redis.NewClient(&redis.Options{
		Addr:         c.CacheRedis[0].Host,
		Password:     c.CacheRedis[0].Pass,
		DB:           0,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	// 初始化 AdminRpc
	adminRpcClient := initAdminRpcClient(c)

	svcCtx := &svc.ServiceContext{
		DB:          db,
		RedisClient: redisClient,
		AdminRpc:    adminRpcClient,
	}

	alertChecker := NewAlertChecker(svcCtx)
	ctx := context.Background()

	// 检查配置 ID 8000000001 的状态
	configID := int64(8000000001)
	result, err := alertChecker.CheckManual(ctx, configID)
	if err != nil {
		t.Fatalf("检查预警状态失败: %v", err)
	}

	t.Logf("预警配置状态:")
	t.Logf("  配置名称: %s", result.ConfigName)
	t.Logf("  当前累计: %s USD", result.TotalAmountUSD)
	t.Logf("  触发阈值: %s USD", result.ThresholdUSD)
	t.Logf("  是否触发: %v", result.Triggered)
	t.Logf("  交易笔数: %d", result.TransactionCount)
	t.Logf("  时间窗口: %s ~ %s", result.WindowStart.Format("15:04:05"), result.WindowEnd.Format("15:04:05"))
	t.Logf("  冷却中: %v", result.InCooldown)
}

// TestLoadConfigsFromAdminRpc 测试从 AdminRpc 加载配置
func TestLoadConfigsFromAdminRpc(t *testing.T) {
	var c businessConfig.Config
	conf.MustLoad("../../etc/business.yaml", &c)

	// 初始化 Redis 客户端
	if len(c.CacheRedis) == 0 {
		t.Fatalf("CacheRedis configuration is empty")
	}
	redisClient := redis.NewClient(&redis.Options{
		Addr:         c.CacheRedis[0].Host,
		Password:     c.CacheRedis[0].Pass,
		DB:           0,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	// 初始化数据库连接
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		c.MySQL.Username, c.MySQL.Password, c.MySQL.Host, c.MySQL.Database)
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}

	// 初始化 AdminRpc
	adminRpcClient := initAdminRpcClient(c)

	// 创建带数据库支持的 ConfigLoader
	configLoader := NewConfigLoaderWithDB(adminRpcClient, redisClient, db)

	ctx := context.Background()

	// 获取配置
	configs, err := configLoader.GetEnabledConfigs(ctx)
	if err != nil {
		t.Fatalf("Failed to load alert configs: %v", err)
	}

	t.Logf("加载到 %d 个启用的预警配置:", len(configs))
	for _, cfg := range configs {
		t.Logf("  [%d] %s - 阈值: %s USD, 时间窗口: %d秒",
			cfg.ID, cfg.Name, cfg.ThresholdUSD.String(), cfg.TimeWindowSeconds)
	}

	// 如果数据库中有配置但未显示，说明数据库查询失败
	if len(configs) == 0 {
		t.Log("没有找到启用的预警配置，请确保数据库中有配置且 enabled=1")
	}
}

// InsertTestAlertConfigs 插入测试用的预警配置
func InsertTestAlertConfigs(db *gorm.DB) error {
	now := time.Now()

	configs := []map[string]interface{}{
		{
			"id":                        int64(8000000001),
			"name":                      "测试预警 - 1分钟10万USD",
			"description":               "测试用：1分钟内交易额超过10万美元触发预警",
			"alert_type":                "platform_transaction",
			"threshold_usd":             100000.0,
			"time_window_seconds":       60, // 1分钟
			"monitor_web3_withdraw":     1,
			"monitor_web2_withdraw":     1,
			"monitor_internal_transfer": 1,
			"enabled":                   1,
			"test_mode":                 0,
			"cooldown_seconds":          300,
			"created_by":                0,
			"updated_by":                0,
			"created_at":                now,
			"updated_at":                now,
		},
		{
			"id":                        int64(8000000002),
			"name":                      "测试预警 - 5分钟50万USD",
			"description":               "测试用：5分钟内交易额超过50万美元触发预警",
			"alert_type":                "platform_transaction",
			"threshold_usd":             500000.0,
			"time_window_seconds":       300, // 5分钟
			"monitor_web3_withdraw":     1,
			"monitor_web2_withdraw":     1,
			"monitor_internal_transfer": 0,
			"enabled":                   0,
			"test_mode":                 0,
			"cooldown_seconds":          600,
			"created_by":                0,
			"updated_by":                0,
			"created_at":                now,
			"updated_at":                now,
		},
	}

	for _, cfg := range configs {
		sql := `INSERT INTO alert_configs (id, name, description, alert_type, threshold_usd, time_window_seconds,
			monitor_web3_withdraw, monitor_web2_withdraw, monitor_internal_transfer, enabled, test_mode, cooldown_seconds,
			created_by, updated_by, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE
			name = VALUES(name),
			description = VALUES(description),
			threshold_usd = VALUES(threshold_usd),
			time_window_seconds = VALUES(time_window_seconds),
			enabled = VALUES(enabled),
			test_mode = VALUES(test_mode),
			cooldown_seconds = VALUES(cooldown_seconds),
			updated_at = NOW()`

		if err := db.Exec(sql,
			cfg["id"], cfg["name"], cfg["description"], cfg["alert_type"], cfg["threshold_usd"], cfg["time_window_seconds"],
			cfg["monitor_web3_withdraw"], cfg["monitor_web2_withdraw"], cfg["monitor_internal_transfer"],
			cfg["enabled"], cfg["test_mode"], cfg["cooldown_seconds"],
			cfg["created_by"], cfg["updated_by"], cfg["created_at"], cfg["updated_at"],
		).Error; err != nil {
			logx.Errorf("Failed to insert alert config %d: %v", cfg["id"], err)
			return err
		}
		logx.Infof("插入测试预警配置: %s (ID=%d)", cfg["name"], cfg["id"])
	}

	return nil
}

// InsertTestTelegramConfig 插入测试用的 Telegram 配置
func InsertTestTelegramConfig(db *gorm.DB) error {
	now := time.Now()

	configs := []struct {
		id       int64
		category string
		keyName  string
		value    interface{}
	}{
		{9000000001, "telegram", "bot_token", "请替换为你的BotToken"},
		{9000000002, "telegram", "chat_id", "请替换为你的ChatID"},
		{9000000003, "telegram", "enabled", false},
		{9000000004, "telegram", "timeout_seconds", 30},
	}

	for _, cfg := range configs {
		valueJSON, _ := json.Marshal(cfg.value)

		sql := `INSERT INTO admin_system_configs (id, category, key_name, value, updated_by, created_at, updated_at)
			VALUES (?, ?, ?, ?, 0, ?, ?)
			ON DUPLICATE KEY UPDATE
			value = VALUES(value),
			updated_at = NOW()`

		if err := db.Exec(sql, cfg.id, cfg.category, cfg.keyName, valueJSON, now, now).Error; err != nil {
			logx.Errorf("Failed to insert telegram config %s: %v", cfg.keyName, err)
		} else {
			logx.Infof("插入 Telegram 配置: %s", cfg.keyName)
		}
	}

	logx.Info("请手动更新 admin_system_configs 表中的 telegram.bot_token 和 telegram.chat_id 为真实值")
	logx.Info("然后执行: UPDATE admin_system_configs SET value = 'true' WHERE category = 'telegram' AND key_name = 'enabled'")

	return nil
}

// printAccumulatedAmount 打印 Redis 中的累计金额
func printAccumulatedAmount(ctx context.Context, redisClient *redis.Client, configID int64) {
	now := time.Now()
	windowStart := now.Truncate(60 * time.Second)
	windowKey := fmt.Sprintf("alert:amount:%d:%d", configID, windowStart.Unix())

	result, err := redisClient.HGetAll(ctx, windowKey).Result()
	if err != nil {
		logx.Infof("Redis key: %s (无数据或错误: %v)", windowKey, err)
		return
	}

	logx.Infof("Redis key: %s", windowKey)
	if totalAmount, ok := result["total_amount_usd"]; ok {
		logx.Infof("  累计金额(USD): %s", totalAmount)
	}
	if txCount, ok := result["transaction_count"]; ok {
		logx.Infof("  交易笔数: %s", txCount)
	}

	// 检查冷却状态
	cooldownKey := fmt.Sprintf("alert:cooldown:%d", configID)
	if cooldown, err := redisClient.Get(ctx, cooldownKey).Result(); err == nil {
		logx.Infof("  冷却状态: 冷却中 (剩余TTL: %s)", cooldown)
	} else {
		logx.Infof("  冷却状态: 未冷却")
	}
}

// maskString 遮盖字符串中间部分
func maskString(s string, showLen int, mask string) string {
	if len(s) <= showLen {
		return s
	}
	runes := []rune(s)
	return string(runes[:showLen]) + mask + string(runes[len(runes)-4:])
}
