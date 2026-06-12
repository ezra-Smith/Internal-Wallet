package notification

import (
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// AlertInfo 预警信息
type AlertInfo struct {
	ConfigName        string
	TotalAmount       decimal.Decimal
	ThresholdAmount   decimal.Decimal
	TxCount           int
	TxCountInt        int64 // 兼容字段
	WindowStart       time.Time
	WindowEnd         time.Time
	Transactions      []TransactionDetail
	MonitorScope      []string
	TimeWindowSeconds int
	EventType         string // 触发的事件类型
}

// TransactionDetail 交易详情
type TransactionDetail struct {
	Timestamp time.Time
	EventType string
	AssetCode string
	Amount    string
	AmountUSD string
}

// BuildAlertMessage 构建预警消息
func BuildAlertMessage(info *AlertInfo) string {
	var sb strings.Builder

	// 标题
	sb.WriteString("🚨 *交易金额预警*\n\n")

	// 规则信息
	sb.WriteString(fmt.Sprintf("*规则名称*: %s\n", info.ConfigName))
	sb.WriteString(fmt.Sprintf("*预警时间*: %s\n\n", time.Now().Format("2006-01-02 15:04:05")))

	// 统计信息
	sb.WriteString("📊 *统计信息*\n")
	sb.WriteString(fmt.Sprintf("• 时间窗口: %s - %s\n",
		info.WindowStart.Format("15:04:05"), info.WindowEnd.Format("15:04:05")))
	sb.WriteString(fmt.Sprintf("• 累计金额: *$%s*\n", formatAmount(info.TotalAmount)))
	sb.WriteString(fmt.Sprintf("• 触发阈值: $%s\n", formatAmount(info.ThresholdAmount)))
	sb.WriteString(fmt.Sprintf("• 交易笔数: %d\n\n", info.TxCount))

	// 配置信息
	sb.WriteString("💡 *配置信息*\n")
	sb.WriteString(fmt.Sprintf("• 监控范围: %s\n", formatMonitorScope(info.MonitorScope)))
	sb.WriteString(fmt.Sprintf("• 时间窗口: %d秒\n\n", info.TimeWindowSeconds))

	// 交易详情(最近5笔)
	if len(info.Transactions) > 0 {
		sb.WriteString("📝 *最近交易*\n")
		maxShow := 5
		if len(info.Transactions) < maxShow {
			maxShow = len(info.Transactions)
		}
		for i := 0; i < maxShow; i++ {
			tx := info.Transactions[i]
			sb.WriteString(fmt.Sprintf("• %s | %s | %s | %s | $%s\n",
				tx.Timestamp.Format("15:04:05"),
				getEventTypeLabel(tx.EventType),
				tx.AssetCode,
				tx.Amount,
				tx.AmountUSD,
			))
		}
		if len(info.Transactions) > maxShow {
			sb.WriteString(fmt.Sprintf("... 还有 %d 笔交易\n", len(info.Transactions)-maxShow))
		}
		sb.WriteString("\n")
	}

	// 页脚
	sb.WriteString("------\n")
	sb.WriteString("此消息由Zink Wallet自动发送")

	return sb.String()
}

// BuildTestMessage 构建测试消息
func BuildTestMessage(customMsg string) string {
	if customMsg != "" {
		return fmt.Sprintf("🧪 *测试通知*\n\n%s\n\n_此消息由Internal Wallet自动发送_", customMsg)
	}

	return "🧪 *测试通知*\n\n如果你看到这条消息，说明Telegram Bot配置正常！\n\n_此消息由Internal Wallet自动发送_"
}

// formatAmount 格式化金额，添加千位分隔符
func formatAmount(amount decimal.Decimal) string {
	// 转换为字符串并添加千位分隔符
	str := amount.String()
	parts := strings.Split(str, ".")
	intPart := parts[0]

	// 添加千位分隔符
	var result []rune
	for i, c := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, c)
	}

	if len(parts) > 1 {
		return string(result) + "." + parts[1]
	}

	return string(result)
}

// formatMonitorScope 格式化监控范围
func formatMonitorScope(scopes []string) string {
	if len(scopes) == 0 {
		return "[无]"
	}
	return fmt.Sprintf("[%s]", strings.Join(scopes, ", "))
}

// getEventTypeLabel 获取事件类型标签
func getEventTypeLabel(eventType string) string {
	switch eventType {
	case "web3_withdraw":
		return "Web3提币"
	case "web2_withdraw":
		return "Web2提币"
	case "internal_transfer":
		return "内部转账"
	default:
		return eventType
	}
}
