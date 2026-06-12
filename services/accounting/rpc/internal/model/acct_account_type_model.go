package model

import (
	"time"

	"gorm.io/gorm"
)

// AcctAccountTypeModel maps to `acct_account_types`.
type AcctAccountTypeModel struct {
	Code        string `gorm:"column:code;type:varchar(64);primaryKey"`
	Name        string `gorm:"column:name;type:varchar(128);not null;default:''"`
	Description string `gorm:"column:description;type:varchar(255);not null;default:''"`
	NormalSide  string `gorm:"column:normal_side;type:varchar(8);not null"`

	CreatedAt time.Time      `gorm:"column:created_at;type:datetime;autoCreateTime"`
	UpdatedAt time.Time      `gorm:"column:updated_at;type:datetime;autoUpdateTime"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (AcctAccountTypeModel) TableName() string { return "acct_account_types" }
