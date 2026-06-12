package logic

import (
	"context"
	"fmt"
	"strings"
	"time"

	"internalwallet/common/notification"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

// VaultBalanceNotificationScheduler 金库余额Telegram通知定时任务
type VaultBalanceNotificationScheduler struct {
	svcCtx   *svc.ServiceContext
	interval time.Duration
}

// NewVaultBalanceNotificationScheduler 创建新的金库余额通知调度器
func NewVaultBalanceNotificationScheduler(svcCtx *svc.ServiceContext, interval time.Duration) *VaultBalanceNotificationScheduler {
	if interval <= 0 {
		interval = 10 * time.Minute // 默认10分钟
	}
	return &VaultBalanceNotificationScheduler{
		svcCtx:   svcCtx,
		interval: interval,
	}
}

// Run 启动定时通知任务
func (s *VaultBalanceNotificationScheduler) Run(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}

	logger := logx.WithContext(ctx)
	logger.Info("金库余额Telegram通知调度器正在初始化...")

	// 详细的依赖检查
	if s == nil || s.svcCtx == nil {
		logger.Error("❌ 金库余额通知调度器启动失败: svcCtx 未配置")
		return
	}
	logger.Info("✓ ServiceContext 已配置")

	if s.svcCtx.DB == nil {
		logger.Error("❌ 金库余额通知调度器启动失败: 数据库(DB)未配置")
		return
	}
	logger.Info("✓ 数据库(DB)已配置")

	if s.svcCtx.VaultNetworkRepo == nil {
		logger.Error("❌ 金库余额通知调度器启动失败: VaultNetworkRepo 未配置")
		return
	}
	logger.Info("✓ VaultNetworkRepo 已配置")

	if s.svcCtx.VaultBalanceRepo == nil {
		logger.Error("❌ 金库余额通知调度器启动失败: VaultBalanceRepo 未配置")
		return
	}
	logger.Info("✓ VaultBalanceRepo 已配置")

	if s.svcCtx.RedisClient == nil {
		logger.Error("❌ 金库余额通知调度器启动失败: Redis 未配置")
		return
	}
	logger.Info("✓ Redis 已配置")

	t := time.NewTicker(s.interval)
	defer t.Stop()

	logger.Info("======================================")
	logger.Infof("✅ 金库余额Telegram通知调度器启动成功!")
	logger.Infof("📊 通知间隔: %s", s.interval)
	logger.Infof("⏰ 下次通知时间: %s", time.Now().Add(s.interval).Format("2006-01-02 15:04:05"))
	logger.Info("======================================")

	// 启动时立即执行一次通知(如果需要可以注释掉)
	//go func() {
	//	logger.Info("🚀 执行首次通知(启动后立即执行)...")
	//	s.sendNotification(ctx)
	//}()

	for {
		select {
		case <-ctx.Done():
			logger.Info("🛑 金库余额通知调度器已停止")
			return
		case <-t.C:
			logger.Infof("⏰ 定时触发: 当前时间=%s", time.Now().Format("2006-01-02 15:04:05"))
			s.sendNotification(ctx)
			logger.Infof("📅 下次通知时间: %s", time.Now().Add(s.interval).Format("2006-01-02 15:04:05"))
		}
	}
}

// sendNotification 发送金库余额通知
func (s *VaultBalanceNotificationScheduler) sendNotification(ctx context.Context) {
	logger := logx.WithContext(ctx)
	startTime := time.Now()

	logger.Info("📨 开始发送金库余额Telegram通知...")
	logger.Infof("⏰ 执行时间: %s", startTime.Format("2006-01-02 15:04:05"))

	// 查询所有启用的网络
	logger.Info("🔍 步骤1: 查询启用的 vault 网络...")
	networks, err := s.svcCtx.VaultNetworkRepo.List(ctx, "active")
	if err != nil {
		logger.Errorf("❌ 查询 vault 网络失败: %v", err)
		return
	}

	if len(networks) == 0 {
		logger.Info("⚠️  没有启用的 vault 网络，跳过通知")
		return
	}
	logger.Infof("✓ 找到 %d 个启用的网络", len(networks))

	// 查询每个网络的余额
	logger.Info("🔍 步骤2: 查询每个网络的余额...")
	balancesMap := make(map[int64][]*model.VaultBalanceModel)
	totalBalanceCount := 0
	for _, network := range networks {
		if network == nil {
			continue
		}
		logger.Infof("   查询网络: %s (ID=%d)", network.Network, network.ID)
		balances, err := s.svcCtx.VaultBalanceRepo.ListByNetworkID(ctx, network.ID)
		if err != nil {
			logger.Errorf("   ❌ 查询网络 %s (ID=%d) 的余额失败: %v", network.Network, network.ID, err)
			continue
		}
		if len(balances) > 0 {
			balancesMap[network.ID] = balances
			totalBalanceCount += len(balances)
			logger.Infof("   ✓ 网络 %s 有 %d 条余额记录", network.Network, len(balances))
		} else {
			logger.Infof("   ⚠️  网络 %s 没有余额记录", network.Network)
		}
	}

	if len(balancesMap) == 0 {
		logger.Info("⚠️  没有金库余额数据，跳过通知")
		return
	}
	logger.Infof("✓ 总共找到 %d 条余额记录", totalBalanceCount)

	// 检查 Telegram 配置是否启用
	logger.Info("🔍 步骤3: 检查Telegram配置...")
	telegramConfig, err := s.getTelegramConfig(ctx)
	if err != nil {
		logger.Errorf("⚠️  无法获取Telegram配置: %v", err)
		logger.Info("   跳过本次通知")
		return
	}

	if !telegramConfig.Enabled {
		logger.Info("⚠️  Telegram通知已禁用 (admin_system_configs.category='telegram', key_name='enabled' = false)")
		logger.Info("   跳过本次通知")
		logger.Info("   💡 如需启用，请在数据库中将 enabled 设置为 true")
		return
	}

	if telegramConfig.BotToken == "" || telegramConfig.ChatID == "" {
		logger.Error("⚠️  Telegram配置不完整:")
		if telegramConfig.BotToken == "" {
			logger.Error("   - bot_token 未配置")
		}
		if telegramConfig.ChatID == "" {
			logger.Error("   - chat_id 未配置")
		}
		logger.Info("   跳过本次通知")
		return
	}

	logger.Infof("✓ Telegram配置已启用 (Bot Token: %s..., Chat ID: %s)",
		maskString(telegramConfig.BotToken, 10),
		telegramConfig.ChatID)

	// 构建消息
	logger.Info("🔍 步骤4: 构建Telegram消息...")
	message := s.buildMessage(networks, balancesMap)
	logger.Infof("✓ 消息构建完成，长度: %d 字符", len(message))
	logger.Infof("消息预览:\n%s", message)

	// 发送到Telegram
	logger.Info("🔍 步骤5: 发送到Telegram...")
	err = notification.SendTelegramMessage(ctx, s.svcCtx.RedisClient, s.svcCtx.DB, message)
	if err != nil {
		logger.Errorf("❌ 发送Telegram通知失败: %v", err)
		return
	}

	elapsed := time.Since(startTime)
	logger.Info("✅ 金库余额Telegram通知发送成功!")
	logger.Infof("⏱️  耗时: %v", elapsed)
}

// buildMessage 构建Telegram消息
func (s *VaultBalanceNotificationScheduler) buildMessage(networks []*model.VaultNetworkModel, balancesMap map[int64][]*model.VaultBalanceModel) string {
	var sb strings.Builder

	// 标题
	sb.WriteString("🏦 *金库余额报告*\n")
	sb.WriteString(fmt.Sprintf("⏰ %s\n", time.Now().Format("2006-01-02 15:04:05")))
	sb.WriteString("━━━━━━━━━━━━━━━\n")

	// 按网络分组显示
	for _, network := range networks {
		if network == nil {
			continue
		}
		balances, ok := balancesMap[network.ID]
		if !ok || len(balances) == 0 {
			continue
		}

		// 网络信息
		sb.WriteString(fmt.Sprintf("* %s 链*\n", escapeMarkdown(network.Network)))
		//if network.VaultAddress != nil && *network.VaultAddress != "" {
		//	sb.WriteString(fmt.Sprintf("📍 地址: `%s`\n\n", maskAddress(*network.VaultAddress)))
		//} else {
		//	sb.WriteString("📍 地址: *未配置*\n\n")
		//}

		// 余额列表
		for _, balance := range balances {
			if balance == nil {
				continue
			}
			// 跳过余额为0的币种
			if balance.Balance == "0" || balance.Balance == "" {
				continue
			}
			currency := escapeMarkdown(balance.Currency)
			balanceStr := escapeMarkdown(balance.Balance)
			balanceUSDStr := escapeMarkdown(balance.BalanceUSD) + " $"
			sb.WriteString(fmt.Sprintf("💰%s:  %s (_%s_)\n", currency, balanceStr, balanceUSDStr))
		}
		sb.WriteString("━━━━━━━━━━━━━━━\n")
	}
	// 页脚
	sb.WriteString("此消息由Zink Wallet自动发送")
	return sb.String()
}

// escapeMarkdown 转义Markdown特殊字符
func escapeMarkdown(text string) string {
	// Telegram Markdown特殊字符: _ * [ ] ( ) ~ ` > # + - = | { } . !
	specialChars := []string{"_", "*", "[", "]", "(", ")", "~", "`", ">", "#", "+"}
	result := text
	for _, char := range specialChars {
		result = strings.ReplaceAll(result, char, "\\"+char)
	}
	return result
}

// getTelegramConfig 获取 Telegram 配置
func (s *VaultBalanceNotificationScheduler) getTelegramConfig(ctx context.Context) (*TelegramConfig, error) {
	cfg := &TelegramConfig{
		Enabled:        false,
		TimeoutSeconds: 30,
	}

	if s.svcCtx.DB == nil {
		return cfg, fmt.Errorf("database not configured")
	}

	// 从数据库查询配置
	var configs []struct {
		Category string `gorm:"column:category"`
		KeyName  string `gorm:"column:key_name"`
		Value    string `gorm:"column:value"`
	}

	err := s.svcCtx.DB.WithContext(ctx).
		Table("admin_system_configs").
		Select("category, key_name, value").
		Where("category = ? AND deleted_at IS NULL", "telegram").
		Find(&configs).Error

	if err != nil {
		return cfg, fmt.Errorf("failed to query telegram configs: %w", err)
	}

	// 解析配置
	for _, item := range configs {
		switch item.KeyName {
		case "bot_token":
			// 去掉JSON字符串的引号
			cfg.BotToken = strings.Trim(item.Value, "\"")
		case "chat_id":
			cfg.ChatID = strings.Trim(item.Value, "\"")
		case "enabled":
			// 解析布尔值
			if item.Value == "true" || item.Value == "\"true\"" {
				cfg.Enabled = true
			}
		case "timeout_seconds":
			// 尝试解析超时时间
			var timeout int
			if _, err := fmt.Sscanf(item.Value, "%d", &timeout); err == nil && timeout > 0 {
				cfg.TimeoutSeconds = timeout
			}
		}
	}

	return cfg, nil
}

// TelegramConfig Telegram配置结构
type TelegramConfig struct {
	BotToken       string
	ChatID         string
	Enabled        bool
	TimeoutSeconds int
}

// maskString 遮蔽字符串，只显示前n个字符
func maskString(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
