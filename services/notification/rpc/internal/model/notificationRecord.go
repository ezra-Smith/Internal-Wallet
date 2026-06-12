package model

import (
	"database/sql/driver"
	"encoding/json"
	"internalwallet/common/model"
	"time"
)

// NotificationRecord 推送记录模型（对应 notification_records 表）
type NotificationRecord struct {
	model.BaseModel

	UserID           int64      `gorm:"column:user_id;not null;index:idx_notification_records_user_id" json:"user_id"`
	DeviceID         int64      `gorm:"column:device_id;not null;default:0;index:idx_notification_records_device_id" json:"device_id"`
	NotificationType string     `gorm:"column:notification_type;size:20;not null;index:idx_notification_records_type" json:"notification_type"`
	TemplateCode     string     `gorm:"column:template_code;size:50;not null;default:''" json:"template_code"`
	Title            string     `gorm:"column:title;size:200;not null" json:"title"`
	Content          string     `gorm:"column:content;type:text;not null" json:"content"`
	Data             JSONMap    `gorm:"column:data;type:json" json:"data"` // 扩展数据（JSON格式）
	PushStatus       string     `gorm:"column:push_status;size:20;not null;default:'pending';index:idx_notification_records_status" json:"push_status"`
	JPushMsgID       string     `gorm:"column:jpush_msg_id;size:50;not null;default:'';index:idx_notification_records_jpush_msg_id" json:"jpush_msg_id"`
	RegistrationID   string     `gorm:"column:registration_id;size:100;not null;default:''" json:"registration_id"`
	ErrorMessage     string     `gorm:"column:error_message;type:text" json:"error_message"`
	SentAt           *time.Time `gorm:"column:sent_at" json:"sent_at"`
	ClickedAt        *time.Time `gorm:"column:clicked_at" json:"clicked_at"`
	RetryCount       int        `gorm:"column:retry_count;not null;default:0" json:"retry_count"`
}

// TableName 指定表名
func (NotificationRecord) TableName() string {
	return "notification_records"
}

// JSONMap 用于存储JSON格式的扩展数据
type JSONMap map[string]interface{}

// Scan 实现 sql.Scanner 接口
func (j *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*j = make(JSONMap)
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		*j = make(JSONMap)
		return nil
	}
	return json.Unmarshal(bytes, j)
}

// Value 实现 driver.Valuer 接口
func (j JSONMap) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// IsPending 判断是否待发送
func (r *NotificationRecord) IsPending() bool {
	return r.PushStatus == "pending"
}

// IsSent 判断是否已发送
func (r *NotificationRecord) IsSent() bool {
	return r.PushStatus == "sent"
}

// IsFailed 判断是否发送失败
func (r *NotificationRecord) IsFailed() bool {
	return r.PushStatus == "failed"
}

// IsClicked 判断是否已点击
func (r *NotificationRecord) IsClicked() bool {
	return r.PushStatus == "clicked"
}

// CanRetry 判断是否可以重试
func (r *NotificationRecord) CanRetry(maxRetries int) bool {
	return r.IsFailed() && r.RetryCount < maxRetries
}
