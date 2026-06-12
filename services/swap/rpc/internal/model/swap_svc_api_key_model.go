package model

import (
	commonModel "internalwallet/common/model"
	"time"
)

// SwapSvcApiKeyModel maps to swap_svc_api_keys.
type SwapSvcApiKeyModel struct {
	commonModel.BaseModel
	KeyHash       string     `gorm:"column:key_hash;type:varchar(128);not null;uniqueIndex"`
	ProjectName   string     `gorm:"column:project_name;type:varchar(128);not null;index"`
	KeyName       string     `gorm:"column:key_name;type:varchar(64);not null;default:''"`
	RateLimitTier string     `gorm:"column:rate_limit_tier;type:varchar(32);not null;default:'default'"`
	IsActive      bool       `gorm:"column:is_active;type:tinyint(1);not null;default:1;index"`
	ExpiresAt     *time.Time `gorm:"column:expires_at;type:datetime"`
	TokenDisplay  string     `gorm:"column:token_display;type:varchar(64);not null;default:''"`
}

func (SwapSvcApiKeyModel) TableName() string { return "swap_svc_api_keys" }
