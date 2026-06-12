package model

import (
	"database/sql/driver"
	"encoding/json"
	commonModel "internalwallet/common/model"
)

// AddressMonitorGorm 地址监控 GORM 模型
type AddressMonitorGorm struct {
	commonModel.BaseModel

	// 监控ID
	MonitorID string `gorm:"column:monitor_id;size:128;not null;uniqueIndex:idx_monitor_id" json:"monitor_id"`

	// 区块链类型
	Chain string `gorm:"column:chain;size:32;not null;index:idx_chain_address,priority:1" json:"chain"`

	// 监控地址
	Address string `gorm:"column:address;size:128;not null;index:idx_chain_address,priority:2" json:"address"`

	// 监控类型：balance, transaction, contract, all
	MonitorType string `gorm:"column:monitor_type;size:32;not null;default:'balance';index:idx_monitor_type" json:"monitor_type"`

	// 优先级：low, normal, high, critical
	Priority string `gorm:"column:priority;size:16;not null;default:'normal'" json:"priority"`

	// 标签
	Tag string `gorm:"column:tag;size:64" json:"tag"`

	// 回调URL
	WebhookURL string `gorm:"column:webhook_url;size:512" json:"webhook_url"`

	// 是否激活：true-激活，false-停用
	Active bool `gorm:"column:active;not null;default:true;index:idx_active" json:"active"`

	// 额外元数据（JSON格式）
	Metadata Metadata `gorm:"column:metadata;type:json" json:"metadata"`

	// 最后活动时间
	LastActivity *int64 `gorm:"column:last_activity" json:"last_activity"`
}

// Metadata 元数据类型
type Metadata map[string]string

// Value 实现 driver.Valuer 接口，用于数据库存储
func (m Metadata) Value() (driver.Value, error) {
	if m == nil {
		return nil, nil
	}
	return json.Marshal(m)
}

// Scan 实现 sql.Scanner 接口，用于数据库读取
func (m *Metadata) Scan(value interface{}) error {
	if value == nil {
		*m = Metadata{}
		return nil
	}

	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, m)
	case string:
		return json.Unmarshal([]byte(v), m)
	default:
		return nil
	}
}

// TableName 指定表名
func (AddressMonitorGorm) TableName() string {
	return "address_monitors"
}
