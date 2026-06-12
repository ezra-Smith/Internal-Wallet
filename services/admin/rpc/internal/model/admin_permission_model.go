package model

import "time"

// AdminPermissionModel maps to admin_permission (RBAC v2).
type AdminPermissionModel struct {
	ID          int64      `gorm:"column:id;primaryKey;autoIncrement"`
	Name        string     `gorm:"column:name;type:varchar(128);not null"`
	Code        string     `gorm:"column:code;type:varchar(128);not null;uniqueIndex"`
	Description *string    `gorm:"column:description;type:varchar(500)"`
	MenuID      *int64     `gorm:"column:menu_id;type:bigint;index"`
	Status      int32      `gorm:"column:status;type:tinyint(1);not null;default:1;index"`
	CreatedAt   *time.Time `gorm:"column:created_at;type:datetime"`
	UpdatedAt   *time.Time `gorm:"column:updated_at;type:datetime"`
	DeletedAt   *time.Time `gorm:"column:deleted_at;type:timestamp"`
}

func (AdminPermissionModel) TableName() string { return "admin_permission" }
