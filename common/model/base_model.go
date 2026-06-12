package model

import (
	"gorm.io/gorm"
	"internalwallet/common/utils"
	"time"
)

// BaseModel 基础 Model，包含所有表的公共字段
// 所有业务 Model 可以继承这个结构
type BaseModel struct {
	ID        int64          `gorm:"column:id;primaryKey;autoIncrement:false" json:"id"` // 雪花ID
	CreatedAt time.Time      `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index" json:"deleted_at"` // NULL=未删除；非 NULL=已删除
	//Version     int       `gorm:"column:version;default:1" json:"version"`       // 乐观锁版本号
	//CreatedUserID int64     `gorm:"column:created_user_id" json:"created_user_id"`
	//UpdatedUserID int64     `gorm:"column:updated_user_id" json:"updated_user_id,omitempty"`
}

// IsDeleted 判断是否已删除
func (m *BaseModel) IsDeleted() bool {
	return m.DeletedAt.Valid
}

// BeforeCreate GORM 钩子：创建前自动生成雪花 ID
// 所有继承 BaseModel 的 Model 都会自动执行此钩子
func (m *BaseModel) BeforeCreate(tx *gorm.DB) error {
	// 如果 ID 为 0，自动生成雪花 ID
	if m.ID == 0 {
		m.ID = utils.GenerateID()
	}
	now := time.Now().Local()
	m.CreatedAt = now
	m.UpdatedAt = now
	m.DeletedAt = gorm.DeletedAt{}
	return nil
}

// BeforeUpdate GORM 钩子：更新前更新更新时间
// 所有继承 BaseModel 的 Model 都会自动执行此钩子
func (m *BaseModel) BeforeUpdate(tx *gorm.DB) error {
	m.UpdatedAt = time.Now().Local()
	return nil
}

// TableName 需要在具体的 Model 中实现
// func (YourModel) TableName() string {
//     return "your_table_name"
// }
