package alert

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
	commonAlert "internalwallet/common/alert"
	"internalwallet/proto/pb"
)

// ConfigLoader 配置加载器
type ConfigLoader struct {
	adminRpc pb.AdminClient
	redis    *redis.Client
	db       *gorm.DB
	cache    []*commonAlert.AlertConfig
	cacheExp time.Time
}

// NewConfigLoader 创建配置加载器
func NewConfigLoader(adminRpc pb.AdminClient, redis *redis.Client) *ConfigLoader {
	return &ConfigLoader{
		adminRpc: adminRpc,
		redis:    redis,
	}
}

// NewConfigLoaderWithDB 创建配置加载器（带数据库支持）
func NewConfigLoaderWithDB(adminRpc pb.AdminClient, redis *redis.Client, db *gorm.DB) *ConfigLoader {
	return &ConfigLoader{
		adminRpc: adminRpc,
		redis:    redis,
		db:       db,
	}
}

// GetEnabledConfigs 获取启用的预警配置
func (l *ConfigLoader) GetEnabledConfigs(ctx context.Context) ([]*commonAlert.AlertConfig, error) {
	// 1. 优先从内存缓存读取
	if l.cache != nil && time.Now().Before(l.cacheExp) {
		logx.Debugf("Using cached alert configs, count=%d", len(l.cache))
		return l.cache, nil
	}

	// 2. 从 Redis 读取缓存的配置
	cached, _ := l.redis.Get(ctx, "alert:config:all").Result()
	if cached != "" {
		var configs []*commonAlert.AlertConfig
		if err := json.Unmarshal([]byte(cached), &configs); err == nil && len(configs) > 0 {
			l.cache = configs
			l.cacheExp = time.Now().Add(5 * time.Minute)
			logx.Infof("Loaded alert configs from Redis cache, count=%d", len(configs))
			return configs, nil
		}
	}

	// 3. 从 Admin RPC 获取（如果可用）
	if l.adminRpc != nil {
		resp, err := l.adminRpc.ListAlertConfigs(ctx, &pb.AdminListAlertConfigsRequest{
			EnabledOnly: true,
			Page:        1,
			PageSize:    100,
		})
		if err != nil {
			logx.Errorf("Failed to get alert configs from Admin RPC: %v", err)
		} else if len(resp.Configs) > 0 {
			configs := l.convertFromProto(resp.Configs)
			if len(configs) > 0 {
				l.cache = configs
				l.cacheExp = time.Now().Add(5 * time.Minute)
				if data, err := json.Marshal(configs); err == nil {
					_ = l.redis.Set(ctx, "alert:config:all", data, 5*time.Minute).Err()
				}
				logx.Infof("Loaded alert configs from Admin RPC, count=%d", len(configs))
				return configs, nil
			}
		}
	}

	// 4. 从数据库直接读取（备用方案）
	if l.db != nil {
		configs, err := l.loadConfigsFromDB(ctx)
		if err != nil {
			logx.Infof("Failed to load configs from DB: %v", err)
		} else if len(configs) > 0 {
			l.cache = configs
			l.cacheExp = time.Now().Add(5 * time.Minute)
			if data, err := json.Marshal(configs); err == nil {
				_ = l.redis.Set(ctx, "alert:config:all", data, 5*time.Minute).Err()
			}
			logx.Infof("Loaded alert configs from database, count=%d", len(configs))
			return configs, nil
		}
	}

	// 5. 全部失败，返回空配置
	logx.Info("No enabled alert configs found (all sources unavailable)")
	return []*commonAlert.AlertConfig{}, nil
}

// loadConfigsFromDB 从数据库加载预警配置
func (l *ConfigLoader) loadConfigsFromDB(ctx context.Context) ([]*commonAlert.AlertConfig, error) {
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

	err := l.db.WithContext(ctx).
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

// RefreshCache 刷新配置缓存
func (l *ConfigLoader) RefreshCache(ctx context.Context) error {
	l.cache = nil
	l.cacheExp = time.Time{}
	_, err := l.GetEnabledConfigs(ctx)
	return err
}

// convertFromProto 从 Proto 转换为 AlertConfig
func (l *ConfigLoader) convertFromProto(
	items []*pb.AdminAlertConfigItem,
) []*commonAlert.AlertConfig {
	configs := make([]*commonAlert.AlertConfig, 0, len(items))
	for _, item := range items {
		threshold, _ := decimal.NewFromString(item.ThresholdUsd)
		config := &commonAlert.AlertConfig{
			ID:                      item.Id,
			Name:                    item.Name,
			Description:             item.Description,
			AlertType:               item.AlertType,
			ThresholdUSD:            threshold,
			TimeWindowSeconds:       int(item.TimeWindowSeconds),
			MonitorWeb3Withdraw:     item.MonitorWeb3Withdraw,
			MonitorWeb2Withdraw:     item.MonitorWeb2Withdraw,
			MonitorInternalTransfer: item.MonitorInternalTransfer,
			CooldownSeconds:         int(item.CooldownSeconds),
		}
		configs = append(configs, config)
	}
	return configs
}

// ShouldMonitor 判断配置是否监控指定事件类型
func ShouldMonitor(config *commonAlert.AlertConfig, eventType string) bool {
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
