package model

import commonModel "internalwallet/common/model"

// UserAdminNoteModel 管理员对用户的备注
type UserAdminNoteModel struct {
	commonModel.BaseModel

	UserID          int64  `gorm:"column:user_id;type:bigint;not null;index"`
	Content         string `gorm:"column:content;type:text;not null"`
	IsImportant     bool   `gorm:"column:is_important;type:tinyint(1);not null;default:0"`
	OperatorAdminID int64  `gorm:"column:operator_admin_id;type:bigint;not null;index"`
}

func (UserAdminNoteModel) TableName() string { return "user_admin_notes" }
