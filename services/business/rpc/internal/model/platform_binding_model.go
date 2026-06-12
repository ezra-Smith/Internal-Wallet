package model

import (
	commonModel "internalwallet/common/model"
	"time"
)

type PlatformBindingModel struct {
	commonModel.BaseModel
	BindingId          string    `gorm:"column:binding_id;type:varchar(32);not null;uniqueIndex"`
	UserId             int64     `gorm:"column:user_id;type:bigint;not null;index"`
	PlatformUid        string    `gorm:"column:platform_uid;type:varchar(32);not null;uniqueIndex"`
	WalletAddress      string    `gorm:"column:wallet_address;type:varchar(100);default:'';index"`
	Chain              string    `gorm:"column:chain;type:varchar(10);default:''"`
	Signature          string    `gorm:"column:signature;type:text"`
	SignMessage        string    `gorm:"column:sign_message;type:text"`
	VerificationMethod int32     `gorm:"column:verification_method;type:tinyint;default:0"`
	Status             int32     `gorm:"column:status;type:tinyint;default:1;index"`
	BoundAt            time.Time `gorm:"column:bound_at;type:datetime"`
	UnboundAt          time.Time `gorm:"column:unbound_at;type:datetime"`
	UnboundReason      string    `gorm:"column:unbound_reason;type:varchar(255);default:''"`
	CreatedAt          time.Time `gorm:"column:created_at;type:datetime"`
	UpdatedAt          time.Time `gorm:"column:updated_at;type:datetime"`
}

func (PlatformBindingModel) TableName() string { return "platform_bindings" }
