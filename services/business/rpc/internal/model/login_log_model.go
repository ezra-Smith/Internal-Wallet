package model

import (
	commonModel "internalwallet/common/model"
)

type LoginLogModel struct {
	commonModel.BaseModel
	UserId     int64  `gorm:"column:user_id;type:bigint;not null;index"`
	DeviceId   string `gorm:"column:device_id;type:varchar(100);default:''"`
	DeviceType string `gorm:"column:device_type;type:varchar(50);default:''"`
	OsVersion  string `gorm:"column:os_version;type:varchar(50);default:''"`
	AppVersion string `gorm:"column:app_version;type:varchar(20);default:''"`
	IpAddress  string `gorm:"column:ip_address;type:varchar(50);default:''"`
	IpLocation string `gorm:"column:ip_location;type:varchar(100);default:''"`
	UserAgent  string `gorm:"column:user_agent;type:text"`
	IsAbnormal bool   `gorm:"column:is_abnormal;type:tinyint(1);default:0"`
	RiskLevel  int32  `gorm:"column:risk_level;type:tinyint;default:0"`
}

func (LoginLogModel) TableName() string { return "login_logs" }
