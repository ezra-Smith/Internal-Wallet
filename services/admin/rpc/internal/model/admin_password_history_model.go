package model

import (
	commonModel "internalwallet/common/model"
)

// AdminPasswordHistoryModel 管理员密码历史（用于禁止最近N次重复）
type AdminPasswordHistoryModel struct {
	commonModel.BaseModel

	AdminID      int64  `gorm:"column:admin_id;type:bigint;not null;index"`
	PasswordHash string `gorm:"column:password_hash;type:varchar(255);not null"`
}

func (AdminPasswordHistoryModel) TableName() string { return "admin_password_histories" }
