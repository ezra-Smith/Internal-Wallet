package model

import (
	"time"

	"github.com/shopspring/decimal"
	commonModel "internalwallet/common/model"
)

// AlertConfigModel 预警配置模型
type AlertConfigModel struct {
	commonModel.BaseModel

	Name                    string          `gorm:"column:name;type:varchar(128);not null"`
	Description             string          `gorm:"column:description;type:text"`
	AlertType               string          `gorm:"column:alert_type;type:varchar(32);not null;default:'platform_transaction'"`
	ThresholdUSD            decimal.Decimal `gorm:"column:threshold_usd;type:decimal(36,18);not null"`
	TimeWindowSeconds       int             `gorm:"column:time_window_seconds;type:int;not null"`
	MonitorWeb3Withdraw     bool            `gorm:"column:monitor_web3_withdraw;type:tinyint(1);not null;default:1"`
	MonitorWeb2Withdraw     bool            `gorm:"column:monitor_web2_withdraw;type:tinyint(1);not null;default:1"`
	MonitorInternalTransfer bool            `gorm:"column:monitor_internal_transfer;type:tinyint(1);not null;default:1"`
	Enabled                 bool            `gorm:"column:enabled;type:tinyint(1);not null;default:1"`
	TestMode                bool            `gorm:"column:test_mode;type:tinyint(1);not null;default:0"`
	CooldownSeconds         int             `gorm:"column:cooldown_seconds;type:int;not null;default:300"`
	CreatedBy               int64           `gorm:"column:created_by;type:bigint;not null;default:0"`
	UpdatedBy               int64           `gorm:"column:updated_by;type:bigint;not null;default:0"`
}

func (AlertConfigModel) TableName() string {
	return "alert_configs"
}

// TransactionAmount 交易金额信息（用于预警监控）
type TransactionAmount struct {
	EventType   string                 `json:"event_type"`   // web3_withdraw, web2_withdraw, internal_transfer
	Timestamp   time.Time              `json:"timestamp"`    // 交易时间
	AssetCode   string                 `json:"asset_code"`   // 资产代码
	Amount      decimal.Decimal        `json:"amount"`       // 原始金额
	AmountUSD   decimal.Decimal        `json:"amount_usd"`   // USD等值
	USDRate     decimal.Decimal        `json:"usd_rate"`     // 汇率
	UserID      int64                  `json:"user_id"`      // 用户ID
	UserType    string                 `json:"user_type"`    // web2, web3, admin
	FromAddress string                 `json:"from_address"` // 来源地址
	ToAddress   string                 `json:"to_address"`   // 目标地址
	Network     string                 `json:"network"`      // 网络
	Metadata    map[string]interface{} `json:"metadata"`     // 额外元数据
}
