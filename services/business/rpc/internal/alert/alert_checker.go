package alert

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
	commonAlert "internalwallet/common/alert"
	"internalwallet/common/notification"
	"internalwallet/services/business/rpc/internal/svc"
)

// AlertChecker 预警检查器
type AlertChecker struct {
	svcCtx       *svc.ServiceContext
	configLoader *ConfigLoader
	accumulator  *commonAlert.Accumulator
}

// NewAlertChecker 创建预警检查器
func NewAlertChecker(svcCtx *svc.ServiceContext) *AlertChecker {
	return &AlertChecker{
		svcCtx: svcCtx,
	}
}

// init 初始化依赖
func (c *AlertChecker) init() {
	if c.configLoader == nil {
		c.configLoader = NewConfigLoaderWithDB(c.svcCtx.AdminRpc, c.svcCtx.RedisClient, c.svcCtx.DB)
	}
	if c.accumulator == nil {
		c.accumulator = commonAlert.NewAccumulator(c.svcCtx.RedisClient)
	}
}

// CheckAndNotify 检查并发送预警通知
// eventType: "web3_withdraw" | "web2_withdraw" | "internal_transfer"
// amount: 原始金额
// amountUSD: USD等值（如果已计算，传入0会自动计算）
func (c *AlertChecker) CheckAndNotify(
	ctx context.Context,
	eventType string,
	assetCode string,
	amount decimal.Decimal,
	amountUSD decimal.Decimal,
	metadata map[string]interface{},
) error {
	c.init()

	// 1. 如果没有传入 USD 金额，需要计算
	if amountUSD.IsZero() || amountUSD.IsNegative() {
		rate, ok := commonAlert.GetAssetPrice(ctx, c.svcCtx.RedisClient, assetCode)
		if !ok {
			logx.Errorf("[Alert] Failed to get exchange rate for %s", assetCode)
			// 汇率获取失败，使用默认值 0 继续预警（避免遗漏大额交易）
			amountUSD = decimal.Zero
		} else {
			amountUSD = amount.Mul(rate)
		}
	}

	logx.Infof("[Alert] Check: event=%s, asset=%s, amount=%s, amount_usd=%s",
		eventType, assetCode, amount.String(), amountUSD.String())

	// 2. 获取启用的预警配置
	configs, err := c.configLoader.GetEnabledConfigs(ctx)
	if err != nil || len(configs) == 0 {
		logx.Debugf("No enabled alert configs or failed to load: %v", err)
		return nil
	}

	// 3. 遍历配置检查
	for _, config := range configs {
		if !ShouldMonitor(config, eventType) {
			continue
		}
		c.checkConfig(ctx, config, eventType, assetCode, amount, amountUSD, metadata)
	}

	return nil
}

// checkConfig 检查单个配置
func (c *AlertChecker) checkConfig(
	ctx context.Context,
	config *commonAlert.AlertConfig,
	eventType string,
	assetCode string,
	amount decimal.Decimal,
	amountUSD decimal.Decimal,
	metadata map[string]interface{},
) {
	// 1. 累加到时间窗口
	totalUSD, txCount, err := c.accumulator.Add(
		ctx,
		config.ID,
		config.TimeWindowSeconds,
		amountUSD,
	)
	if err != nil {
		logx.Errorf("Failed to accumulate amount for config %d: %v", config.ID, err)
		return
	}

	// 2. 检查是否触发阈值
	if totalUSD.LessThan(config.ThresholdUSD) {
		return
	}

	logx.Infof("[Alert] Threshold exceeded: config=%s, total=%s, threshold=%s",
		config.Name, totalUSD.String(), config.ThresholdUSD.String())

	// 3. 检查冷却时间
	if c.accumulator.IsInCooldown(ctx, config.ID) {
		logx.Infof("[Alert] Config %s is in cooldown, skipping", config.Name)
		return
	}

	// 4. 发送预警通知
	c.sendAlert(ctx, config, eventType, totalUSD, txCount)

	// 5. 设置冷却时间
	if err := c.accumulator.SetCooldown(ctx, config.ID, config.CooldownSeconds); err != nil {
		logx.Errorf("Failed to set cooldown for config %d: %v", config.ID, err)
	}
}

// sendAlert 发送预警通知
func (c *AlertChecker) sendAlert(
	ctx context.Context,
	config *commonAlert.AlertConfig,
	eventType string,
	totalUSD decimal.Decimal,
	txCount int64,
) {
	logx.Infof("[Alert] Sending notification: config=%s, total=%s, threshold=%s",
		config.Name, totalUSD.String(), config.ThresholdUSD.String())

	// 获取 Telegram 配置
	telegramCfg, err := c.getTelegramConfig(ctx)
	if err != nil {
		logx.Errorf("Failed to get telegram config: %v", err)
		return
	}

	if !telegramCfg.Enabled {
		logx.Info("Telegram is disabled, skipping notification")
		return
	}

	// 构建 Telegram 消息
	now := time.Now()
	windowStart := now.Truncate(time.Duration(config.TimeWindowSeconds) * time.Second)
	windowEnd := windowStart.Add(time.Duration(config.TimeWindowSeconds) * time.Second)

	alertInfo := config.ToNotificationInfo(
		totalUSD,
		config.ThresholdUSD,
		txCount,
		windowStart,
		windowEnd,
		eventType,
	)

	message := notification.BuildAlertMessage(alertInfo)

	// 发送消息
	client := notification.NewTelegramClient(
		telegramCfg.BotToken,
		telegramCfg.ChatID,
		time.Duration(telegramCfg.TimeoutSeconds)*time.Second,
	)

	if err := client.SendMessage(message); err != nil {
		logx.Errorf("Failed to send telegram notification: %v", err)
		return
	}

	logx.Infof("[Alert] Notification sent successfully for config %s", config.Name)
}

// getTelegramConfig 获取 Telegram 配置
func (c *AlertChecker) getTelegramConfig(ctx context.Context) (*notification.TelegramConfig, error) {
	cfg := &notification.TelegramConfig{
		Enabled:        false,
		TimeoutSeconds: 30,
	}

	// 先尝试从 Redis 缓存获取
	cacheKey := "system:config:telegram:cached"
	cached, err := c.svcCtx.RedisClient.Get(ctx, cacheKey).Result()
	if err == nil && cached != "" {
		if err := parseTelegramConfigFromJSON([]byte(cached), cfg); err == nil {
			return cfg, nil
		}
	}

	// 从数据库获取
	configs, err := c.getTelegramConfigsFromDB(ctx)
	if err != nil {
		logx.Errorf("Failed to get telegram config from db: %v", err)
		return cfg, nil
	}

	// 解析配置
	for _, item := range configs {
		switch item.KeyName {
		case "bot_token":
			var token string
			if err := json.Unmarshal(item.Value, &token); err == nil {
				cfg.BotToken = token
			}
		case "chat_id":
			var chatID string
			if err := json.Unmarshal(item.Value, &chatID); err == nil {
				cfg.ChatID = chatID
			}
		case "enabled":
			var enabled bool
			if err := json.Unmarshal(item.Value, &enabled); err == nil {
				cfg.Enabled = enabled
			}
		case "timeout_seconds":
			var timeout int
			if err := json.Unmarshal(item.Value, &timeout); err == nil && timeout > 0 {
				cfg.TimeoutSeconds = timeout
			}
		}
	}

	// 缓存到 Redis
	if cfg.BotToken != "" && cfg.ChatID != "" {
		if data, err := json.Marshal(map[string]interface{}{
			"bot_token":       cfg.BotToken,
			"chat_id":         cfg.ChatID,
			"enabled":         cfg.Enabled,
			"timeout_seconds": cfg.TimeoutSeconds,
		}); err == nil {
			c.svcCtx.RedisClient.Set(ctx, cacheKey, data, 5*time.Minute)
		}
	}

	return cfg, nil
}

// parseTelegramConfigFromJSON 从 JSON 解析 Telegram 配置
func parseTelegramConfigFromJSON(data []byte, cfg *notification.TelegramConfig) error {
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}
	if botToken, ok := parsed["bot_token"].(string); ok {
		cfg.BotToken = botToken
	}
	if chatID, ok := parsed["chat_id"].(string); ok {
		cfg.ChatID = chatID
	}
	if enabled, ok := parsed["enabled"].(bool); ok {
		cfg.Enabled = enabled
	}
	if timeout, ok := parsed["timeout_seconds"].(float64); ok {
		cfg.TimeoutSeconds = int(timeout)
	}
	return nil
}

// adminSystemConfigItem 系统配置项（用于数据库查询）
type adminSystemConfigItem struct {
	Category  string `gorm:"column:category"`
	KeyName   string `gorm:"column:key_name"`
	Value     []byte `gorm:"column:value"`
	UpdatedAt string `gorm:"column:updated_at"`
}

// getTelegramConfigsFromDB 从数据库获取 telegram 配置
func (c *AlertChecker) getTelegramConfigsFromDB(ctx context.Context) ([]*adminSystemConfigItem, error) {
	if c.svcCtx.DB == nil {
		return nil, fmt.Errorf("database not available")
	}

	var items []*adminSystemConfigItem
	err := c.svcCtx.DB.WithContext(ctx).
		Table("admin_system_configs").
		Where("category = ? AND deleted_at IS NULL", "telegram").
		Find(&items).Error
	return items, err
}

// CheckManual 手动检查（用于测试）
func (c *AlertChecker) CheckManual(
	ctx context.Context,
	configID int64,
) (*AlertCheckResult, error) {
	c.init()

	configs, err := c.configLoader.GetEnabledConfigs(ctx)
	if err != nil {
		return nil, err
	}

	var targetConfig *commonAlert.AlertConfig
	for _, cfg := range configs {
		if cfg.ID == configID {
			targetConfig = cfg
			break
		}
	}

	if targetConfig == nil {
		return nil, fmt.Errorf("config not found: %d", configID)
	}

	totalUSD, txCount, _ := c.accumulator.GetTotal(ctx, configID, targetConfig.TimeWindowSeconds)

	now := time.Now()
	windowStart := now.Truncate(time.Duration(targetConfig.TimeWindowSeconds) * time.Second)
	windowEnd := windowStart.Add(time.Duration(targetConfig.TimeWindowSeconds) * time.Second)

	return &AlertCheckResult{
		ConfigID:         configID,
		ConfigName:       targetConfig.Name,
		TotalAmountUSD:   totalUSD.String(),
		ThresholdUSD:     targetConfig.ThresholdUSD.String(),
		Triggered:        totalUSD.GreaterThanOrEqual(targetConfig.ThresholdUSD),
		TransactionCount: int(txCount),
		WindowStart:      windowStart,
		WindowEnd:        windowEnd,
		InCooldown:       c.accumulator.IsInCooldown(ctx, configID),
	}, nil
}

// AlertCheckResult 预警检查结果
type AlertCheckResult struct {
	ConfigID         int64
	ConfigName       string
	TotalAmountUSD   string
	ThresholdUSD     string
	Triggered        bool
	TransactionCount int
	WindowStart      time.Time
	WindowEnd        time.Time
	InCooldown       bool
}

// GetConfigLoader 获取配置加载器（用于测试）
func (c *AlertChecker) GetConfigLoader() *ConfigLoader {
	c.init()
	return c.configLoader
}

// GetAccumulator 获取累加器（用于测试）
func (c *AlertChecker) GetAccumulator() *commonAlert.Accumulator {
	c.init()
	return c.accumulator
}
