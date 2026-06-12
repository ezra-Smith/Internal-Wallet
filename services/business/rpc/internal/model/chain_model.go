package model

import (
	commonModel "internalwallet/common/model"
)

type ChainModel struct {
	commonModel.BaseModel
	Name        string `gorm:"column:name;type:varchar(32);not null;uniqueIndex;comment:链名"`
	Network     string `gorm:"column:network;type:varchar(32);default:'';comment:网络名称"`
	ChainID     int    `gorm:"column:chain_id;type:int;default:0;comment:链ID"`
	ExplorerUrl string `gorm:"column:explorer_url;type:varchar(255);default:'';comment:区块浏览器URL"`
	IconUrl     string `gorm:"column:icon_url;type:varchar(2048);default:'';comment:链图标URL"`
	Status      int32  `gorm:"column:status;type:tinyint;default:1;comment:状态:1正常 2禁用"`
}

func (ChainModel) TableName() string { return "chain" }
