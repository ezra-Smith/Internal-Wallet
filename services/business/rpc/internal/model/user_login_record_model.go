package model

import (
	commonModel "internalwallet/common/model"
	"time"
)

type UserLoginRecordModel struct {
	commonModel.BaseModel
	UserId     int64     `gorm:"column:user_id;type:bigint;not null;index;comment:用户ID"`
	DeviceName string    `gorm:"column:device_name;type:varchar(100);default:'';comment:设备名称"`
	Platform   int32     `gorm:"column:platform;type:tinyint;default:0;comment:平台"`
	IpAddress  string    `gorm:"column:ip_address;type:varchar(45);default:'';comment:IP地址"`
	Location   string    `gorm:"column:location;type:varchar(100);default:'';comment:登录位置"`
	LoginTime  time.Time `gorm:"column:login_time;type:datetime;comment:登录时间"`
	Result     string    `gorm:"column:result;type:varchar(20);default:'';comment:登录结果(success/failed)"`
	IsCurrent  bool      `gorm:"column:is_current;type:tinyint(1);default:0;comment:是否当前"`
	IsTrusted  bool      `gorm:"column:is_trusted;type:tinyint(1);default:0;comment:是否可信"`
}

func (UserLoginRecordModel) TableName() string { return "user_login_record" }
