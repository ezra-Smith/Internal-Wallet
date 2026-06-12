package model

import commonModel "internalwallet/common/model"

// UserKycModel 用户实名信息（简化版）
type UserKycModel struct {
	commonModel.BaseModel

	UserId   int64  `gorm:"column:user_id;type:bigint;not null;uniqueIndex;comment:用户ID"`
	RealName string `gorm:"column:real_name;type:varchar(50);default:'';comment:真实姓名"`
	IdNumber string `gorm:"column:id_number;type:varchar(64);default:'';comment:证件号"`
	Birthday string `gorm:"column:birthday;type:varchar(20);default:'';comment:生日"`
	Gender   int32  `gorm:"column:gender;type:tinyint;default:0;comment:性别(0未知 1男 2女)"`
	Address  string `gorm:"column:address;type:varchar(255);default:'';comment:地址"`
}

func (UserKycModel) TableName() string { return "user_kyc" }
