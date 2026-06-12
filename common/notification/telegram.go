package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

const (
	// TelegramAPIBaseURL Telegram Bot API基础URL
	TelegramAPIBaseURL = "https://api.telegram.org"
	// SendMessagePath 发送消息路径
	SendMessagePath = "/sendMessage"
)

// TelegramClient Telegram Bot客户端
type TelegramClient struct {
	botToken   string
	chatID     string
	httpClient *http.Client
	timeout    time.Duration
}

// TelegramConfig Telegram配置
type TelegramConfig struct {
	BotToken       string `json:"bot_token"`
	ChatID         string `json:"chat_id"`
	Enabled        bool   `json:"enabled"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

// NewTelegramClient 创建Telegram Bot客户端
func NewTelegramClient(botToken, chatID string, timeout time.Duration) *TelegramClient {
	if timeout <= 0 {
		timeout = 30 * time.Second // 默认30秒超时
	}

	return &TelegramClient{
		botToken: botToken,
		chatID:   chatID,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		timeout: timeout,
	}
}

// SendMessage 发送消息
func (c *TelegramClient) SendMessage(message string) error {
	if c.botToken == "" || c.chatID == "" {
		return &ErrTelegramNotConfigured{Field: "bot_token or chat_id"}
	}

	// 构建请求
	payload := map[string]interface{}{
		"chat_id":    c.chatID,
		"text":       message,
		"parse_mode": "Markdown", // 支持Markdown格式
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// 发送HTTP请求
	url := fmt.Sprintf("%s/bot%s%s", TelegramAPIBaseURL, c.botToken, SendMessagePath)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return &ErrTelegramSendFailed{Reason: err.Error()}
	}
	defer resp.Body.Close()

	// 检查响应
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return &ErrTelegramSendFailed{
			Reason: fmt.Sprintf("status %d: %s", resp.StatusCode, string(respBody)),
		}
	}

	return nil
}

// TestConnection 测试连接
func (c *TelegramClient) TestConnection() error {
	if c.botToken == "" {
		return &ErrTelegramNotConfigured{Field: "bot_token"}
	}

	url := fmt.Sprintf("%s/bot%s/getMe", TelegramAPIBaseURL, c.botToken)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram connection test failed with status %d", resp.StatusCode)
	}

	return nil
}

// Validate 验证Telegram配置
func (cfg *TelegramConfig) Validate() error {
	if cfg.BotToken == "" {
		return &ErrTelegramNotConfigured{Field: "bot_token"}
	}
	if cfg.ChatID == "" {
		return &ErrTelegramNotConfigured{Field: "chat_id"}
	}
	return nil
}

// ErrTelegramNotConfigured Telegram未配置错误
type ErrTelegramNotConfigured struct {
	Field string
}

func (e *ErrTelegramNotConfigured) Error() string {
	return fmt.Sprintf("telegram %s not configured", e.Field)
}

// ErrTelegramSendFailed Telegram发送失败错误
type ErrTelegramSendFailed struct {
	Reason string
}

func (e *ErrTelegramSendFailed) Error() string {
	return fmt.Sprintf("telegram send failed: %s", e.Reason)
}

// SendTelegramMessage 直接发送 Telegram 消息（自动从配置读取）
// 这是一个便捷方法，会自动从 Redis/DB 获取 Telegram 配置并发送消息
// 适用于需要直接发送消息而不经过预警配置系统的场景
func SendTelegramMessage(ctx context.Context, rdb *redis.Client, db *gorm.DB, message string) error {
	if rdb == nil {
		return fmt.Errorf("redis client is required")
	}

	// 从 Redis 缓存获取配置
	cfg, err := getTelegramConfig(ctx, rdb, db)
	if err != nil {
		return fmt.Errorf("failed to get telegram config: %w", err)
	}

	if !cfg.Enabled {
		return fmt.Errorf("telegram is disabled in configuration (category='telegram', key_name='enabled')")
	}

	if cfg.BotToken == "" {
		return &ErrTelegramNotConfigured{Field: "bot_token (category='telegram', key_name='bot_token')"}
	}

	if cfg.ChatID == "" {
		return &ErrTelegramNotConfigured{Field: "chat_id (category='telegram', key_name='chat_id')"}
	}

	// 创建客户端并发送消息
	client := NewTelegramClient(cfg.BotToken, cfg.ChatID, time.Duration(cfg.TimeoutSeconds)*time.Second)
	return client.SendMessage(message)
}

// SendTelegramMessageSimple 发送简单的 Telegram 文本消息
// 这是 SendTelegramMessage 的简化版本，只传入 Redis Client 和消息文本
func SendTelegramMessageSimple(ctx context.Context, rdb *redis.Client, message string) error {
	return SendTelegramMessage(ctx, rdb, nil, message)
}

// getTelegramConfig 获取 Telegram 配置（从缓存或数据库）
func getTelegramConfig(ctx context.Context, rdb *redis.Client, db *gorm.DB) (*TelegramConfig, error) {
	cfg := &TelegramConfig{
		Enabled:        false,
		TimeoutSeconds: 30,
	}

	// 先尝试从 Redis 缓存获取
	cacheKey := "system:config:telegram:cached"
	cached, err := rdb.Get(ctx, cacheKey).Result()
	if err == nil && cached != "" {
		if err := parseTelegramConfigFromJSON([]byte(cached), cfg); err == nil {
			return cfg, nil
		}
	}

	// 如果有 DB，从数据库获取
	if db != nil {
		configs, err := getTelegramConfigsFromDB(ctx, db)
		if err != nil {
			return cfg, nil // 返回默认配置，不阻塞
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
				rdb.Set(ctx, cacheKey, data, 5*time.Minute)
			}
		}
	}

	return cfg, nil
}

// parseTelegramConfigFromJSON 从 JSON 解析 Telegram 配置
func parseTelegramConfigFromJSON(data []byte, cfg *TelegramConfig) error {
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
func getTelegramConfigsFromDB(ctx context.Context, db *gorm.DB) ([]*adminSystemConfigItem, error) {
	var items []*adminSystemConfigItem
	err := db.WithContext(ctx).
		Table("admin_system_configs").
		Where("category = ? AND deleted_at IS NULL", "telegram").
		Find(&items).Error
	return items, err
}
