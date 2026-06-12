package alert

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
	commonAlert "internalwallet/common/alert"
	"internalwallet/common/notification"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/svc"
)

// CheckAndNotify 检查并发送预警通知
// 这个函数用于 admin 服务中（如手动审核内部转账后触发预警）
func CheckAndNotify(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	eventType string,
	assetCode string,
	amount decimal.Decimal,
	metadata map[string]interface{},
) error {
	if svcCtx == nil || svcCtx.RedisClient == nil {
		return fmt.Errorf("svcCtx or RedisClient not available")
	}

	// 1. 如果没有传入 USD 金额，需要计算
	amountUSD := decimal.Zero
	if amount.IsPositive() {
		rate, ok := commonAlert.GetAssetPrice(ctx, svcCtx.RedisClient, assetCode)
		if !ok {
			logx.Errorf("[Alert] Failed to get exchange rate for %s", assetCode)
		} else {
			amountUSD = amount.Mul(rate)
		}
	}

	logx.Infof("[Alert] Admin Check: event=%s, asset=%s, amount=%s, amount_usd=%s",
		eventType, assetCode, amount.String(), amountUSD.String())

	// 2. 从数据库加载预警配置
	configs, err := loadAlertConfigsFromDB(ctx, svcCtx.DB)
	if err != nil || len(configs) == 0 {
		logx.Debugf("No enabled alert configs or failed to load: %v", err)
		return nil
	}

	// 3. 遍历配置检查
	accumulator := commonAlert.NewAccumulator(svcCtx.RedisClient)
	for _, config := range configs {
		if !shouldMonitorConfig(config, eventType) {
			continue
		}
		checkAndNotifyConfig(ctx, svcCtx, accumulator, config, eventType, assetCode, amount, amountUSD, metadata)
	}

	return nil
}

// loadAlertConfigsFromDB 从数据库加载预警配置
func loadAlertConfigsFromDB(ctx context.Context, db *gorm.DB) ([]*commonAlert.AlertConfig, error) {
	if db == nil {
		return nil, fmt.Errorf("database not available")
	}

	var results []struct {
		ID                      int64   `gorm:"column:id"`
		Name                    string  `gorm:"column:name"`
		Description             string  `gorm:"column:description"`
		AlertType               string  `gorm:"column:alert_type"`
		ThresholdUSD            float64 `gorm:"column:threshold_usd"`
		TimeWindowSeconds       int     `gorm:"column:time_window_seconds"`
		MonitorWeb3Withdraw     int     `gorm:"column:monitor_web3_withdraw"`
		MonitorWeb2Withdraw     int     `gorm:"column:monitor_web2_withdraw"`
		MonitorInternalTransfer int     `gorm:"column:monitor_internal_transfer"`
		CooldownSeconds         int     `gorm:"column:cooldown_seconds"`
	}

	err := db.WithContext(ctx).
		Table("alert_configs").
		Where("enabled = 1 AND deleted_at IS NULL").
		Find(&results).Error
	if err != nil {
		return nil, err
	}

	configs := make([]*commonAlert.AlertConfig, 0, len(results))
	for _, r := range results {
		configs = append(configs, &commonAlert.AlertConfig{
			ID:                      r.ID,
			Name:                    r.Name,
			Description:             r.Description,
			AlertType:               r.AlertType,
			ThresholdUSD:            decimal.NewFromFloat(r.ThresholdUSD),
			TimeWindowSeconds:       r.TimeWindowSeconds,
			MonitorWeb3Withdraw:     r.MonitorWeb3Withdraw == 1,
			MonitorWeb2Withdraw:     r.MonitorWeb2Withdraw == 1,
			MonitorInternalTransfer: r.MonitorInternalTransfer == 1,
			CooldownSeconds:         r.CooldownSeconds,
		})
	}

	return configs, nil
}

// shouldMonitorConfig 判断配置是否监控指定事件类型
func shouldMonitorConfig(config *commonAlert.AlertConfig, eventType string) bool {
	switch eventType {
	case "web3_withdraw":
		return config.MonitorWeb3Withdraw
	case "web2_withdraw":
		return config.MonitorWeb2Withdraw
	case "internal_transfer":
		return config.MonitorInternalTransfer
	default:
		return false
	}
}

// checkAndNotifyConfig 检查单个配置并发送通知
func checkAndNotifyConfig(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	accumulator *commonAlert.Accumulator,
	config *commonAlert.AlertConfig,
	eventType string,
	assetCode string,
	amount decimal.Decimal,
	amountUSD decimal.Decimal,
	metadata map[string]interface{},
) {
	// 1. 累加到时间窗口
	totalUSD, txCount, err := accumulator.Add(
		ctx,
		config.ID,
		config.TimeWindowSeconds,
		amountUSD,
	)
	if err != nil {
		logx.Errorf("[Alert] Failed to accumulate amount for config %d: %v", config.ID, err)
		return
	}

	// 2. 检查是否触发阈值
	if totalUSD.LessThan(config.ThresholdUSD) {
		return
	}

	logx.Infof("[Alert] Threshold exceeded: config=%s, total=%s, threshold=%s",
		config.Name, totalUSD.String(), config.ThresholdUSD.String())

	// 3. 检查冷却时间
	if accumulator.IsInCooldown(ctx, config.ID) {
		logx.Infof("[Alert] Config %s is in cooldown, skipping", config.Name)
		return
	}

	// 4. 发送预警通知
	sendAlert(ctx, svcCtx, config, eventType, totalUSD, txCount)

	// 5. 设置冷却时间
	if err := accumulator.SetCooldown(ctx, config.ID, config.CooldownSeconds); err != nil {
		logx.Errorf("[Alert] Failed to set cooldown for config %d: %v", config.ID, err)
	}
}

// sendAlert 发送预警通知
func sendAlert(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	config *commonAlert.AlertConfig,
	eventType string,
	totalUSD decimal.Decimal,
	txCount int64,
) {
	logx.Infof("[Alert] Sending notification: config=%s, total=%s, threshold=%s",
		config.Name, totalUSD.String(), config.ThresholdUSD.String())

	// 获取 Telegram 配置
	telegramCfg, err := getTelegramConfig(ctx, svcCtx)
	if err != nil {
		logx.Errorf("[Alert] Failed to get telegram config: %v", err)
		return
	}

	if !telegramCfg.Enabled {
		logx.Info("[Alert] Telegram is disabled, skipping notification")
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
		logx.Errorf("[Alert] Failed to send telegram notification: %v", err)
		return
	}

	logx.Infof("[Alert] Notification sent successfully for config %s", config.Name)
}

// getTelegramConfig 获取 Telegram 配置
func getTelegramConfig(ctx context.Context, svcCtx *svc.ServiceContext) (*notification.TelegramConfig, error) {
	cfg := &notification.TelegramConfig{
		Enabled:        false,
		TimeoutSeconds: 30,
	}

	// 先尝试从 Redis 缓存获取
	if svcCtx.RedisClient != nil {
		cacheKey := "system:config:telegram:cached"
		cached, err := svcCtx.RedisClient.Get(ctx, cacheKey).Result()
		if err == nil && cached != "" {
			if err := parseTelegramConfigFromJSON([]byte(cached), cfg); err == nil {
				return cfg, nil
			}
		}

		// 从数据库获取
		configs, err := getTelegramConfigsFromDB(ctx, svcCtx.DB)
		if err != nil {
			logx.Errorf("[Alert] Failed to get telegram config from db: %v", err)
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
				svcCtx.RedisClient.Set(ctx, cacheKey, data, 5*time.Minute)
			}
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

// getTelegramConfigsFromDB 从数据库获取 telegram 配置
func getTelegramConfigsFromDB(ctx context.Context, db *gorm.DB) ([]*adminSystemConfigItem, error) {
	if db == nil {
		return nil, fmt.Errorf("database not available")
	}

	var items []*adminSystemConfigItem
	err := db.WithContext(ctx).
		Table("admin_system_configs").
		Where("category = ? AND deleted_at IS NULL", "telegram").
		Find(&items).Error
	return items, err
}

// adminSystemConfigItem 系统配置项（用于数据库查询）
type adminSystemConfigItem struct {
	Category  string `gorm:"column:category"`
	KeyName   string `gorm:"column:key_name"`
	Value     []byte `gorm:"column:value"`
	UpdatedAt string `gorm:"column:updated_at"`
}

// CheckAndNotifyForTransfer 为内部转账订单触发预警（便捷方法）
func CheckAndNotifyForTransfer(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	order *model.CurrencyTransferOrderModel,
) error {
	amount, err := decimal.NewFromString(order.Amount)
	if err != nil {
		logx.Errorf("[Alert] Failed to parse transfer amount: %v", err)
		return err
	}

	return CheckAndNotify(
		ctx,
		svcCtx,
		"internal_transfer",
		order.AssetCode,
		amount,
		map[string]interface{}{
			"order_id":       order.ID,
			"from_user_id":   order.FromUserID,
			"to_user_id":     order.ToUserID,
			"strategy":       order.Strategy,
			"audit_admin_id": order.AuditAdminID,
		},
	)
}
