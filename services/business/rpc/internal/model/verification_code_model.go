package model

import (
	commonModel "internalwallet/common/model"
	"time"
)

type VerificationCodeModel struct {
	commonModel.BaseModel
	Target     string    `gorm:"column:target;type:varchar(100);default:''"`
	TargetType int32     `gorm:"column:target_type;type:tinyint;default:0"`
	CodeType   int32     `gorm:"column:code_type;type:tinyint;default:0"`
	Code       string    `gorm:"column:code;type:varchar(10);default:''"`
	IsUsed     bool      `gorm:"column:is_used;type:tinyint(1);default:0"`
	ExpireAt   time.Time `gorm:"column:expire_at;type:datetime"`
	CreatedAt  time.Time `gorm:"column:created_at;type:datetime"`
}

func (VerificationCodeModel) TableName() string { return "verification_codes" }
