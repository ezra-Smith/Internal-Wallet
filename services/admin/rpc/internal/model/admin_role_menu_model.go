package model

import "time"

// AdminRoleMenuModel maps to admin_role_menu (RBAC v2).
type AdminRoleMenuModel struct {
	ID        int64      `gorm:"column:id;primaryKey;autoIncrement"`
	RoleID    int64      `gorm:"column:role_id;type:bigint;not null;index"`
	MenuID    int64      `gorm:"column:menu_id;type:bigint;not null;index"`
	CreatedAt *time.Time `gorm:"column:created_at;type:datetime"`
}

func (AdminRoleMenuModel) TableName() string { return "admin_role_menu" }
