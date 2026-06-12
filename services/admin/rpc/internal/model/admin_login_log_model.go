package model

import (
	commonModel "internalwallet/common/model"
)

// AdminLoginLogModel 管理员登录/登出日志
type AdminLoginLogModel struct {
	commonModel.BaseModel

	AdminID int64 `gorm:"column:admin_id;type:bigint;not null;index"`
	// login/logout/login_failed
	Action string `gorm:"column:action;type:varchar(16);not null;index"`

	IP        string `gorm:"column:ip;type:varchar(45);not null"`
	UserAgent string `gorm:"column:user_agent;type:varchar(512);default:''"`
	Location  string `gorm:"column:location;type:varchar(128);default:''"`

	// success/failed
	Result string `gorm:"column:result;type:varchar(16);not null;index"`
	Reason string `gorm:"column:reason;type:varchar(256);default:''"`
}

func (AdminLoginLogModel) TableName() string { return "admin_login_logs" }
