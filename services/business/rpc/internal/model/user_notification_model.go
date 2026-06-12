package model

import (
	"time"

	commonModel "internalwallet/common/model"
)

type UserNotificationModel struct {
	commonModel.BaseModel

	// 用户信息
	UserId int64 `gorm:"column:user_id;type:bigint;not null;index:idx_user_read;comment:用户ID"`

	// 消息内容
	Type    string `gorm:"column:type;type:varchar(32);not null;index;comment:消息类型"`
	Title   string `gorm:"column:title;type:varchar(100);not null;comment:标题"`
	Content string `gorm:"column:content;type:text;not null;comment:内容"`
	Data    string `gorm:"column:data;type:text;comment:额外数据（JSON字符串）"`

	// 状态
	Read      bool       `gorm:"column:read;type:tinyint(1);default:0;index:idx_user_read;comment:是否已读"`
	ReadAt    *time.Time `gorm:"column:read_at;type:timestamp;comment:已读时间"`
	Timestamp int64      `gorm:"column:timestamp;type:bigint;default:0;comment:业务时间戳（秒）"`
}

func (UserNotificationModel) TableName() string { return "user_notification" }
