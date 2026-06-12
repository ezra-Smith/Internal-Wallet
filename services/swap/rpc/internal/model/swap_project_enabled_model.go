package model

import (
	commonModel "internalwallet/common/model"
)

// SwapProjectEnabledModel 项目启用配置模型（项目级开关）
// 对应数据库表：swap_project_enabled
type SwapProjectEnabledModel struct {
	commonModel.BaseModel

	ProjectName string `gorm:"column:project_name;type:varchar(128);not null;index:idx_project" json:"project_name"`
	ConfigID    int64  `gorm:"column:config_id;type:bigint;not null;index:idx_config" json:"config_id"`
	IsEnabled   bool   `gorm:"column:is_enabled;type:tinyint(1);not null;default:1" json:"is_enabled"`
}

// TableName 指定表名
func (SwapProjectEnabledModel) TableName() string {
	return "swap_project_enabled"
}
