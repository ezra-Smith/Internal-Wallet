package model

import commonModel "internalwallet/common/model"

// UserWhitelistSettingsModel 用户白名单设置（当前用于：绕过提款审计）
type UserWhitelistSettingsModel struct {
	commonModel.BaseModel

	UserID int64 `gorm:"column:user_id;type:bigint;not null;uniqueIndex"`

	BypassWithdrawAudit bool   `gorm:"column:bypass_withdraw_audit;type:tinyint(1);not null;default:0;index"`
	Reason              string `gorm:"column:reason;type:varchar(255);not null;default:''"`
	OperatorAdminID     int64  `gorm:"column:operator_admin_id;type:bigint;not null;default:0;index"`

	// Source: admin | user (reserved)
	Source         string `gorm:"column:source;type:varchar(16);not null;default:'admin';index"`
	OperatorUserID int64  `gorm:"column:operator_user_id;type:bigint;not null;default:0;index"`
}

func (UserWhitelistSettingsModel) TableName() string { return "user_whitelist_settings" }
