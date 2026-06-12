package model

import (
	commonModel "internalwallet/common/model"
)

type UserWalletAddressModel struct {
	commonModel.BaseModel
	// NOTE: 用户"提币地址簿"（仅用于 UI 保存，不参与充值地址分配/不作为提币强校验）。
	UserId    int64  `gorm:"column:user_id;type:bigint;not null;index;comment:用户ID"`
	Asset     string `gorm:"column:asset;type:varchar(32);default:'';index;comment:资产代码"`
	Chain     string `gorm:"column:chain;type:varchar(32);default:'';index;comment:链名"`
	Address   string `gorm:"column:address;type:varchar(255);default:'';index;comment:提币地址"`
	Label     string `gorm:"column:label;type:varchar(100);default:'';comment:标签"`
	MemoTag   string `gorm:"column:memo_tag;type:varchar(64);default:'';comment:Memo/Tag"`
	IsDefault bool   `gorm:"column:is_default;type:tinyint(1);default:0;comment:是否默认"`
}

func (UserWalletAddressModel) TableName() string { return "member_wallet_address" }
