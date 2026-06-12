package model

import (
	commonModel "internalwallet/common/model"
	"time"
)

type UserSessionModel struct {
	commonModel.BaseModel
	UserId       int64     `gorm:"column:user_id;type:bigint;not null;index"`
	SessionToken string    `gorm:"column:session_token;type:varchar(255);default:'';uniqueIndex"`
	RefreshToken string    `gorm:"column:refresh_token;type:varchar(255);default:'';uniqueIndex"`
	Platform     int32     `gorm:"column:platform;type:tinyint;default:0"`
	DeviceId     string    `gorm:"column:device_id;type:varchar(100);default:''"`
	DeviceType   string    `gorm:"column:device_type;type:varchar(50);default:''"`
	IpAddress    string    `gorm:"column:ip_address;type:varchar(50);default:''"`
	IpLocation   string    `gorm:"column:ip_location;type:varchar(100);default:''"`
	IsActive     bool      `gorm:"column:is_active;type:tinyint(1);default:0"`
	IsRememberMe bool      `gorm:"column:is_remember_me;type:tinyint(1);default:0"`
	ExpiresAt    time.Time `gorm:"column:expires_at;type:datetime"`
	CreatedAt    time.Time `gorm:"column:created_at;type:datetime"`
}

func (UserSessionModel) TableName() string { return "user_sessions" }
