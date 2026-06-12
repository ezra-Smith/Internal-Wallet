package model

import (
	"database/sql/driver"
	"encoding/json"
	commonModel "internalwallet/common/model"
)

// ProviderStatusGorm 服务商状态 GORM 模型
type ProviderStatusGorm struct {
	commonModel.BaseModel

	// 服务商ID
	ProviderID string `gorm:"column:provider_id;size:128;not null;uniqueIndex:idx_provider_chain,priority:1" json:"provider_id"`

	// 服务商类型
	ProviderType string `gorm:"column:provider_type;size:32;not null;index:idx_provider_type" json:"provider_type"`

	// 区块链类型
	Chain string `gorm:"column:chain;size:32;not null;uniqueIndex:idx_provider_chain,priority:2;index:idx_chain_healthy,priority:1" json:"chain"`

	// 服务商名称
	Name string `gorm:"column:name;size:64;not null" json:"name"`

	// 端点URL
	Endpoint string `gorm:"column:endpoint;size:512;not null" json:"endpoint"`

	// 是否健康：true-健康，false-不健康
	Healthy bool `gorm:"column:healthy;not null;default:true;index:idx_chain_healthy,priority:2" json:"healthy"`

	// 响应时间（毫秒）
	ResponseTimeMs int64 `gorm:"column:response_time_ms;default:0" json:"response_time_ms"`

	// 成功率（百分比）
	SuccessRate int `gorm:"column:success_rate;default:100" json:"success_rate"`

	// 总请求数
	TotalRequests int64 `gorm:"column:total_requests;default:0" json:"total_requests"`

	// 失败请求数
	FailedRequests int64 `gorm:"column:failed_requests;default:0" json:"failed_requests"`

	// 最后成功时间
	LastSuccessTime *int64 `gorm:"column:last_success_time" json:"last_success_time"`

	// 最后错误时间
	LastErrorTime *int64 `gorm:"column:last_error_time" json:"last_error_time"`

	// 最后错误信息
	LastError string `gorm:"column:last_error;type:text" json:"last_error"`

	// 是否为主要服务商
	IsPrimary bool `gorm:"column:is_primary;not null;default:false" json:"is_primary"`

	// 权重
	Weight float64 `gorm:"column:weight;type:decimal(5,2);default:1.00" json:"weight"`

	// 配置信息（JSON格式）
	Config ProviderConfig `gorm:"column:config;type:json" json:"config"`
}

// ProviderConfig 服务商配置类型
type ProviderConfig struct {
	APIKey     string            `json:"api_key,omitempty"`
	SecretKey  string            `json:"secret_key,omitempty"`
	Timeout    int               `json:"timeout,omitempty"`
	RateLimit  int               `json:"rate_limit,omitempty"`
	MaxRetries int               `json:"max_retries,omitempty"`
	RetryDelay int               `json:"retry_delay,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
}

// Value 实现 driver.Valuer 接口，用于数据库存储
func (p ProviderConfig) Value() (driver.Value, error) {
	if p.APIKey == "" && p.SecretKey == "" {
		return nil, nil
	}
	return json.Marshal(p)
}

// Scan 实现 sql.Scanner 接口，用于数据库读取
func (p *ProviderConfig) Scan(value interface{}) error {
	if value == nil {
		*p = ProviderConfig{}
		return nil
	}

	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, p)
	case string:
		return json.Unmarshal([]byte(v), p)
	default:
		return nil
	}
}

// TableName 指定表名
func (ProviderStatusGorm) TableName() string {
	return "provider_status"
}
