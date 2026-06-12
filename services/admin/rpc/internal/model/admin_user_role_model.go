package model

import "time"

// AdminUserRoleModel maps to admin_user_role (RBAC v2).
type AdminUserRoleModel struct {
	ID        int64      `gorm:"column:id;primaryKey;autoIncrement"`
	UserID    int64      `gorm:"column:user_id;type:bigint;not null;index"`
	RoleID    int64      `gorm:"column:role_id;type:bigint;not null;index"`
	CreatedAt *time.Time `gorm:"column:created_at;type:datetime"`
}

func (AdminUserRoleModel) TableName() string { return "admin_user_role" }
