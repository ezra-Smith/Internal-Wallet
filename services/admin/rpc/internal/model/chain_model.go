package model

import (
	commonModel "internalwallet/common/model"
)

// ChainModel maps to `chain` (business-facing chains table).
type ChainModel struct {
	commonModel.BaseModel

	Name        string `gorm:"column:name;type:varchar(32);not null;index" json:"name"` // code (TRON/ETH/...)
	Network     string `gorm:"column:network;type:varchar(32);default:''" json:"network"`
	ChainID     int32  `gorm:"column:chain_id;type:int;default:0" json:"chain_id"`
	ExplorerUrl string `gorm:"column:explorer_url;type:varchar(255);default:''" json:"explorer_url"`
	IconUrl     string `gorm:"column:icon_url;type:varchar(2048);default:''" json:"icon_url"`
	Status      int32  `gorm:"column:status;type:tinyint;default:1;index" json:"status"` // 1=enabled 2=disabled
}

func (ChainModel) TableName() string { return "chain" }
