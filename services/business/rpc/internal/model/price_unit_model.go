package model

import (
	commonModel "internalwallet/common/model"
)

type PriceUnitModel struct {
	commonModel.BaseModel
	Name   string `gorm:"column:name;type:varchar(64);default:''"`
	Status int32  `gorm:"column:status;type:tinyint;default:1"`
}

func (PriceUnitModel) TableName() string { return "price_units" }
