package model

import (
	commonModel "internalwallet/common/model"
)

// AdminSystemConfigModel 系统配置（按 category + key 维度存储）
type AdminSystemConfigModel struct {
	commonModel.BaseModel

	Category string `gorm:"column:category;type:varchar(32);not null;index"`
	KeyName  string `gorm:"column:key_name;type:varchar(64);not null;index"`
	// JSON 值（使用 []byte 存储，便于保存 string/bool/number/object）
	Value []byte `gorm:"column:value;type:json"`

	UpdatedBy int64 `gorm:"column:updated_by;type:bigint;default:0"`
}

func (AdminSystemConfigModel) TableName() string { return "admin_system_configs" }
