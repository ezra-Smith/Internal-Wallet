package model

import (
	commonModel "internalwallet/common/model"
)

// GeetestValidationLogModel records Geetest validation attempts.
type GeetestValidationLogModel struct {
	commonModel.BaseModel
	UserId     int64  `gorm:"column:user_id;type:bigint;not null;default:0;index"`
	Scene      string `gorm:"column:scene;type:varchar(64);not null;index"`
	Identifier string `gorm:"column:identifier;type:varchar(128);default:''"`
	LotNumber  string `gorm:"column:lot_number;type:varchar(128);default:''"`
	IpAddress  string `gorm:"column:ip_address;type:varchar(45);default:'';index"`
	UserAgent  string `gorm:"column:user_agent;type:varchar(512);default:''"`
	RequestId  string `gorm:"column:request_id;type:varchar(64);default:'';index"`
	Success    bool   `gorm:"column:success;type:tinyint(1);not null;default:0;index"`
	FailOpen   bool   `gorm:"column:fail_open;type:tinyint(1);not null;default:0"`
	Reason     string `gorm:"column:reason;type:varchar(255);default:''"`
	Error      string `gorm:"column:error;type:varchar(512);default:''"`
}

func (GeetestValidationLogModel) TableName() string { return "geetest_validation_log" }
