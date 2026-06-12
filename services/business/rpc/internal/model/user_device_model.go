package model

import (
	commonModel "internalwallet/common/model"
	"time"
)

type UserDeviceModel struct {
	commonModel.BaseModel
	UserId            int64     `gorm:"column:user_id;type:bigint;not null;index;comment:用户ID"`
	DeviceName        string    `gorm:"column:device_name;type:varchar(512);default:'';comment:设备名称"`
	Platform          int32     `gorm:"column:platform;type:tinyint;default:0;comment:平台"`
	LastLoginIp       string    `gorm:"column:last_login_ip;type:varchar(45);default:'';comment:最后登录IP"`
	LastLoginLocation string    `gorm:"column:last_login_location;type:varchar(100);default:'';comment:最后登录位置"`
	LastLoginTime     time.Time `gorm:"column:last_login_time;type:datetime;comment:最后登录时间"`
	IsCurrent         bool      `gorm:"column:is_current;type:tinyint(1);default:0;comment:是否当前设备"`
	IsTrusted         bool      `gorm:"column:is_trusted;type:tinyint(1);default:0;comment:是否可信设备"`
	LoginCount        int32     `gorm:"column:login_count;type:int;default:0;comment:登录次数"`
	FirstLoginTime    time.Time `gorm:"column:first_login_time;type:datetime;comment:首次登录时间"`
}

func (UserDeviceModel) TableName() string { return "user_device" }
