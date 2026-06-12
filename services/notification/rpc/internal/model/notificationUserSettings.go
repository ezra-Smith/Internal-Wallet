package model

import (
	"database/sql/driver"
	"internalwallet/common/model"
	"time"
)

// NotificationUserSettings 用户推送设置模型（对应 notification_user_settings 表）
type NotificationUserSettings struct {
	model.BaseModel

	UserID            int64    `gorm:"column:user_id;not null;uniqueIndex:uk_notification_user_settings_user_id" json:"user_id"`
	EnableTransaction bool     `gorm:"column:enable_transaction;not null;default:true" json:"enable_transaction"`
	EnableSecurity    bool     `gorm:"column:enable_security;not null;default:true" json:"enable_security"`
	EnableSystem      bool     `gorm:"column:enable_system;not null;default:true" json:"enable_system"`
	EnablePriceAlert  bool     `gorm:"column:enable_price_alert;not null;default:true" json:"enable_price_alert"`
	QuietStartTime    NullTime `gorm:"column:quiet_start_time;type:time" json:"quiet_start_time"` // 免打扰开始时间
	QuietEndTime      NullTime `gorm:"column:quiet_end_time;type:time" json:"quiet_end_time"`     // 免打扰结束时间
	Language          string   `gorm:"column:language;size:10;not null;default:'zh-CN'" json:"language"`
}

// TableName 指定表名
func (NotificationUserSettings) TableName() string {
	return "notification_user_settings"
}

// NullTime 用于处理可为空的 TIME 类型
type NullTime struct {
	Time  time.Time
	Valid bool // Valid is true if Time is not NULL
}

// Scan 实现 sql.Scanner 接口
func (nt *NullTime) Scan(value interface{}) error {
	if value == nil {
		nt.Time, nt.Valid = time.Time{}, false
		return nil
	}
	nt.Valid = true
	switch v := value.(type) {
	case time.Time:
		nt.Time = v
	case []byte:
		t, err := time.Parse("15:04:05", string(v))
		if err != nil {
			return err
		}
		nt.Time = t
	case string:
		t, err := time.Parse("15:04:05", v)
		if err != nil {
			return err
		}
		nt.Time = t
	}
	return nil
}

// Value 实现 driver.Valuer 接口
func (nt NullTime) Value() (driver.Value, error) {
	if !nt.Valid {
		return nil, nil
	}
	return nt.Time.Format("15:04:05"), nil
}

// IsNotificationEnabled 判断是否启用了指定类型的通知
func (s *NotificationUserSettings) IsNotificationEnabled(notificationType string) bool {
	switch notificationType {
	case "transaction":
		return s.EnableTransaction
	case "security":
		return s.EnableSecurity
	case "system":
		return s.EnableSystem
	case "price_alert":
		return s.EnablePriceAlert
	default:
		return true
	}
}

// IsInQuietTime 判断当前时间是否在免打扰时段
func (s *NotificationUserSettings) IsInQuietTime(now time.Time) bool {
	if !s.QuietStartTime.Valid || !s.QuietEndTime.Valid {
		return false
	}

	currentTime := now.Format("15:04:05")
	startTime := s.QuietStartTime.Time.Format("15:04:05")
	endTime := s.QuietEndTime.Time.Format("15:04:05")

	// 处理跨天的情况（例如 23:00 到 07:00）
	if startTime > endTime {
		return currentTime >= startTime || currentTime <= endTime
	}

	return currentTime >= startTime && currentTime <= endTime
}
