package alert

import (
	"time"

	"github.com/shopspring/decimal"
	"internalwallet/common/notification"
)

// AlertConfig 预警配置
type AlertConfig struct {
	ID                      int64
	Name                    string
	Description             string
	AlertType               string
	ThresholdUSD            decimal.Decimal
	TimeWindowSeconds       int
	MonitorWeb3Withdraw     bool
	MonitorWeb2Withdraw     bool
	MonitorInternalTransfer bool
	CooldownSeconds         int
}

// TransactionDetail 交易详情（用于通知展示）
type TransactionDetail struct {
	Timestamp time.Time
	EventType string
	AssetCode string
	Amount    decimal.Decimal
	AmountUSD decimal.Decimal
}

// ToNotificationInfo 转换 AlertConfig 为 notification.AlertInfo
func (c *AlertConfig) ToNotificationInfo(
	totalAmount decimal.Decimal,
	thresholdAmount decimal.Decimal,
	txCount int64,
	windowStart, windowEnd time.Time,
	eventType string,
) *notification.AlertInfo {
	return &notification.AlertInfo{
		ConfigName:        c.Name,
		TotalAmount:       totalAmount,
		ThresholdAmount:   thresholdAmount,
		TxCount:           int(txCount),
		TxCountInt:        txCount,
		WindowStart:       windowStart,
		WindowEnd:         windowEnd,
		Transactions:      nil, // TODO: 添加交易详情
		MonitorScope:      c.GetMonitorScope(),
		TimeWindowSeconds: c.TimeWindowSeconds,
		EventType:         eventType,
	}
}

// GetMonitorScope 获取监控范围
func (c *AlertConfig) GetMonitorScope() []string {
	var scope []string
	if c.MonitorWeb3Withdraw {
		scope = append(scope, "Web3提现")
	}
	if c.MonitorWeb2Withdraw {
		scope = append(scope, "Web2提现")
	}
	if c.MonitorInternalTransfer {
		scope = append(scope, "内部转账")
	}
	return scope
}

// CheckInput 预警检查输入参数
type CheckInput struct {
	EventType string
	AssetCode string
	Amount    decimal.Decimal
	AmountUSD decimal.Decimal
	Metadata  map[string]interface{}
}
