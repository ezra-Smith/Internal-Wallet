package model

import (
	"database/sql/driver"
	"encoding/json"

	commonModel "internalwallet/common/model"
)

// AddressMonitorModel 地址监控 GORM 模型（address_monitors 表）
//
// 说明：
// - 该表由 chainsync 侧消费并维护内存监控集合，但 business 侧也会做“兜底登记”（best-effort），
//   以保证某些入口（如 Web3 资产总览）不会因为缺少监控记录而影响后续链上监听。
// - 由于各服务是独立编译单元，这里需要在 business 服务内提供该表的最小模型定义。
type AddressMonitorModel struct {
	commonModel.BaseModel

	// 监控ID（全局唯一）
	MonitorID string `gorm:"column:monitor_id;size:128;not null;uniqueIndex:idx_monitor_id" json:"monitor_id"`

	// 区块链类型（如 ethereum / bsc / tron）
	Chain string `gorm:"column:chain;size:32;not null;index:idx_chain_address,priority:1" json:"chain"`

	// 监控地址（建议归一化后存储）
	Address string `gorm:"column:address;size:128;not null;index:idx_chain_address,priority:2" json:"address"`

	// 监控类型：balance / transaction / contract / all
	MonitorType string `gorm:"column:monitor_type;size:32;not null;default:'balance';index:idx_monitor_type" json:"monitor_type"`

	// 优先级：low / normal / high / critical
	Priority string `gorm:"column:priority;size:16;not null;default:'normal'" json:"priority"`

	// 标签（用于区分来源/用途）
	Tag string `gorm:"column:tag;size:64" json:"tag"`

	// 回调URL（可选）
	WebhookURL string `gorm:"column:webhook_url;size:512" json:"webhook_url"`

	// 是否激活：true-激活，false-停用
	Active bool `gorm:"column:active;not null;default:true;index:idx_active" json:"active"`

	// 额外元数据（JSON）
	Metadata AddressMonitorMetadata `gorm:"column:metadata;type:json" json:"metadata"`

	// 最后活动时间（epoch seconds/millis 由上游约定）
	LastActivity *int64 `gorm:"column:last_activity" json:"last_activity"`
}

// AddressMonitorMetadata 元数据类型（JSON）
type AddressMonitorMetadata map[string]string

// Value 实现 driver.Valuer 接口（写入 DB）
func (m AddressMonitorMetadata) Value() (driver.Value, error) {
	if m == nil {
		return nil, nil
	}
	return json.Marshal(m)
}

// Scan 实现 sql.Scanner 接口（从 DB 读取）
func (m *AddressMonitorMetadata) Scan(value interface{}) error {
	if value == nil {
		*m = AddressMonitorMetadata{}
		return nil
	}

	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, m)
	case string:
		return json.Unmarshal([]byte(v), m)
	default:
		// 保持容错：未知类型直接忽略
		return nil
	}
}

func (AddressMonitorModel) TableName() string { return "address_monitors" }

