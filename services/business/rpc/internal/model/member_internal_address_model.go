package model

import (
	commonModel "internalwallet/common/model"
)

// MemberInternalAddressModel 用户内部地址簿（平台内转账目标用户）
// 说明:
// - 用于保存平台内其他用户作为转账目标（通过 UID/邮箱添加）
// - 与 member_wallet_address（链上提币地址）是独立的概念
// - 内部转账不走链，无需资产/链/地址等字段
type MemberInternalAddressModel struct {
	commonModel.BaseModel
	UserID            int64  `gorm:"column:user_id;type:bigint;not null;index;comment:用户ID（地址所有者）"`
	TargetUserID      int64  `gorm:"column:target_user_id;type:bigint;not null;index;comment:目标用户ID"`
	TargetUserDisplay string `gorm:"column:target_user_display;type:varchar(255);default:'';comment:目标用户显示标识"`
	Label             string `gorm:"column:label;type:varchar(100);default:'';comment:标签/备注名"`
	Remark            string `gorm:"column:remark;type:varchar(255);default:'';comment:UID备注"`
	IsDefault         bool   `gorm:"column:is_default;type:tinyint(1);default:0;comment:是否默认"`
}

func (MemberInternalAddressModel) TableName() string { return "member_internal_address" }

