package model

import (
	commonModel "internalwallet/common/model"
	"time"
)

type BlacklistModel struct {
	commonModel.BaseModel
	Identifier      string    `gorm:"column:identifier;type:varchar(255);not null;index;uniqueIndex:uniq_identifier_type"`
	IdentifierType  int32     `gorm:"column:identifier_type;type:tinyint;not null;default:1;index;uniqueIndex:uniq_identifier_type"`
	UserId          int64     `gorm:"column:user_id;type:bigint;default:0;index"`
	ReasonType      int32     `gorm:"column:reason_type;type:tinyint;default:0"`
	ReasonDetail    string    `gorm:"column:reason_detail;type:text"`
	Evidence        string    `gorm:"column:evidence;type:json"`
	BlockedFeatures string    `gorm:"column:blocked_features;type:json"`
	CanAppeal       bool      `gorm:"column:can_appeal;type:tinyint(1);default:1"`
	AppealCount     int       `gorm:"column:appeal_count;type:int;default:0"`
	OperatorId      int64     `gorm:"column:operator_id;type:bigint;default:0"`
	OperatorName    string    `gorm:"column:operator_name;type:varchar(100)"`
	ExpiresAt       time.Time `gorm:"column:expires_at;type:datetime"`
	Status          int32     `gorm:"column:status;type:tinyint;not null;default:1;index"`
	CreatedAt       time.Time `gorm:"column:created_at;type:datetime;not null;autoCreateTime"`
	UpdatedAt       time.Time `gorm:"column:updated_at;type:datetime;not null;autoUpdateTime"`
}

func (BlacklistModel) TableName() string { return "blacklists" }
