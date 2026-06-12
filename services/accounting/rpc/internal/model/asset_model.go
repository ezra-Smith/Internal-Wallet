package model

import (
	"time"

	"gorm.io/gorm"
)

// AssetModel maps to `asset` reference table (business-facing assets).
// NOTE: Soft-delete is based on `deleted_at`, so enforce uniqueness only for active rows in DB (see init-db SQL).
type AssetModel struct {
	ID        int64          `gorm:"column:id;primaryKey"`
	CreatedAt time.Time      `gorm:"column:created_at;type:datetime;autoCreateTime"`
	UpdatedAt time.Time      `gorm:"column:updated_at;type:datetime;autoUpdateTime"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index"`

	Code      string `gorm:"column:code;type:varchar(32);not null;index"`
	Name      string `gorm:"column:name;type:varchar(64);not null;default:''"`
	IconUrl   string `gorm:"column:icon_url;type:varchar(2048);not null;default:''"`
	IsHot     bool   `gorm:"column:is_hot;type:tinyint;not null;default:0"`
	Precision int32  `gorm:"column:precision;type:int;not null;default:18"`
	Status    int8   `gorm:"column:status;type:tinyint;not null;default:1"`
}

func (AssetModel) TableName() string { return "asset" }
