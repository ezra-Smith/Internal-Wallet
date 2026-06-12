package model

import (
	commonModel "internalwallet/common/model"
)

type UserSecuritySettingsModel struct {
	commonModel.BaseModel
	UserId            int64  `gorm:"column:user_id;type:bigint;not null;uniqueIndex;comment:用户ID"`
	GoogleAuthEnabled bool   `gorm:"column:google_auth_enabled;type:tinyint(1);default:0;comment:是否开启GA"`
	GoogleAuthBound   bool   `gorm:"column:google_auth_bound;type:tinyint(1);default:0;comment:是否绑定了Google Authenticator"`
	HasTradePassword  bool   `gorm:"column:has_trade_password;type:tinyint(1);default:0;comment:是否设置资金密码"`
	AntiPhishingCode  string `gorm:"column:anti_phishing_code;type:varchar(20);default:'';comment:防钓鱼码"`
	EmailBound        bool   `gorm:"column:email_bound;type:tinyint(1);default:0;comment:邮箱已绑定"`
	PhoneBound        bool   `gorm:"column:phone_bound;type:tinyint(1);default:0;comment:手机号已绑定"`
	LoginNotification bool   `gorm:"column:login_notification;type:tinyint(1);default:0;comment:登录通知"`
	EmailMasked       string `gorm:"column:email_masked;type:varchar(255);default:'';comment:邮箱脱敏"`
	PhoneMasked       string `gorm:"column:phone_masked;type:varchar(50);default:'';comment:手机号脱敏"`
}

func (UserSecuritySettingsModel) TableName() string { return "member_security_setting" }
