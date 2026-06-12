package model

import (
	commonModel "internalwallet/common/model"
	"time"
)

type UserRoleModel struct {
	commonModel.BaseModel
	UserId      int64     `gorm:"column:user_id;type:bigint;not null;index;uniqueIndex:uniq_user_role"`
	RoleType    int32     `gorm:"column:role_type;type:tinyint;default:0;index;uniqueIndex:uniq_user_role"`
	Permissions string    `gorm:"column:permissions;type:json"`
	AssignedBy  int64     `gorm:"column:assigned_by;type:bigint;default:0"`
	AssignedAt  time.Time `gorm:"column:assigned_at;type:datetime"`
	ExpiresAt   time.Time `gorm:"column:expires_at;type:datetime"`
	Status      int32     `gorm:"column:status;type:tinyint;default:1"`
	CreatedAt   time.Time `gorm:"column:created_at;type:datetime"`
	UpdatedAt   time.Time `gorm:"column:updated_at;type:datetime"`
}

func (UserRoleModel) TableName() string { return "user_roles" }
