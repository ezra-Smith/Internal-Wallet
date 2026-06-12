package model

import "time"

// AdminRolePermissionModel maps to admin_role_permission (RBAC v2).
type AdminRolePermissionModel struct {
	ID           int64      `gorm:"column:id;primaryKey;autoIncrement"`
	RoleID       int64      `gorm:"column:role_id;type:bigint;not null;index"`
	PermissionID int64      `gorm:"column:permission_id;type:bigint;not null;index"`
	CreatedAt    *time.Time `gorm:"column:created_at;type:datetime"`
}

func (AdminRolePermissionModel) TableName() string { return "admin_role_permission" }
