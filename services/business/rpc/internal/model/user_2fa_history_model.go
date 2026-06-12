package model

import commonModel "internalwallet/common/model"

// User2FAHistoryModel 用户 2FA 操作历史（TOTP）
type User2FAHistoryModel struct {
	commonModel.BaseModel

	UserID int64 `gorm:"column:user_id;type:bigint;not null;index"`

	// bind | unbind | rebind | admin_unbind
	Event string `gorm:"column:event;type:varchar(32);not null;index"`
	// Current system only supports TOTP.
	Factor string `gorm:"column:factor;type:varchar(32);not null;default:'totp';index"`

	// operator_type: user | admin | system
	OperatorType string `gorm:"column:operator_type;type:varchar(16);not null;index"`
	OperatorID   int64  `gorm:"column:operator_id;type:bigint;not null;index"`

	Reason    string `gorm:"column:reason;type:varchar(255);not null;default:''"`
	IP        string `gorm:"column:ip;type:varchar(45);not null;default:''"`
	UserAgent string `gorm:"column:user_agent;type:varchar(512);not null;default:''"`

	// Meta JSON (serialized bytes).
	Meta []byte `gorm:"column:meta;type:json"`
}

func (User2FAHistoryModel) TableName() string { return "user_2fa_history" }
