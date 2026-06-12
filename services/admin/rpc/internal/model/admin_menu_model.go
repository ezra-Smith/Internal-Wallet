package model

import "time"

// AdminMenuModel maps to admin_menu (RBAC v2).
// NOTE: This table schema is managed by SQL migrations (do not rely on AutoMigrate).
type AdminMenuModel struct {
	ID                 int64      `gorm:"column:id;primaryKey;autoIncrement"`
	Pid                int64      `gorm:"column:pid;type:bigint;default:0;index:pid"`
	Level              int        `gorm:"column:level;type:int;not null;default:1"`
	Tree               *string    `gorm:"column:tree;type:text"`
	Name               string     `gorm:"column:name;type:varchar(128);not null"`
	Path               *string    `gorm:"column:path;type:varchar(200)"`
	Icon               *string    `gorm:"column:icon;type:varchar(128)"`
	HideInMenu         bool       `gorm:"column:hide_in_menu;type:tinyint(1);default:0"`
	HideChildrenInMenu bool       `gorm:"column:hide_childrenIn_menu;type:tinyint(1);default:0"`
	Sort               int        `gorm:"column:sort;type:int;default:0"`
	Remark             *string    `gorm:"column:remark;type:varchar(255)"`
	Status             int32      `gorm:"column:status;type:tinyint(1);not null;default:1;index:status"`
	UpdatedAt          *time.Time `gorm:"column:updated_at;type:datetime"`
	CreatedAt          *time.Time `gorm:"column:created_at;type:datetime"`
	DeletedAt          *time.Time `gorm:"column:deleted_at;type:timestamp"`
	Target             *string    `gorm:"column:target;type:varchar(255)"`
	Access             *string    `gorm:"column:access;type:varchar(255)"`
	Key                *string    `gorm:"column:key;type:varchar(255)"`
}

func (AdminMenuModel) TableName() string { return "admin_menu" }
