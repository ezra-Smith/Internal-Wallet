package model

import (
	"time"

	"gorm.io/gorm"
)

// UserModel Web2 用户（与 business service 共用 users 表）
// NOTE: This model shares the same `users` table with business service
// The ID field is now AUTO_INCREMENT (not Snowflake)
type UserModel struct {
	// NOTE: ID field is now AUTO_INCREMENT in database (not Snowflake)
	ID        int64      `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"` // Database auto-increment ID
	CreatedAt time.Time  `gorm:"column:created_at;type:datetime;default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time  `gorm:"column:updated_at;type:datetime;default:CURRENT_TIMESTAMP"`
	DeletedAt *time.Time `gorm:"column:deleted_at;type:datetime;default:NULL;index"` // Soft delete

	Email       string `gorm:"column:email;type:varchar(100);default:'';index"`
	Phone       string `gorm:"column:phone;type:varchar(20);default:'';index"`
	CountryCode string `gorm:"column:country_code;type:varchar(10);default:''"`

	Nickname string `gorm:"column:nickname;type:varchar(50);default:''"`
	Avatar   string `gorm:"column:avatar;type:varchar(255);default:''"`

	KycLevel    int32 `gorm:"column:kyc_level;type:tinyint;default:0;index"`
	MemberLevel int32 `gorm:"column:member_level;type:tinyint;default:1;index"`

	Is2faEnabled bool `gorm:"column:is_2fa_enabled;type:tinyint(1);default:0"`

	// status: 1-active 2-frozen 3-terminated
	Status int32 `gorm:"column:status;type:tinyint;default:1;index"`

	RegisterIp    string    `gorm:"column:register_ip;type:varchar(50);default:''"`
	LastLoginTime time.Time `gorm:"column:last_login_time;type:datetime"`
	LastLoginIp   string    `gorm:"column:last_login_ip;type:varchar(45);default:''"`
}

func (UserModel) TableName() string { return "users" }

// BeforeCreate hook - do NOT generate Snowflake ID (use database AUTO_INCREMENT)
func (u *UserModel) BeforeCreate(tx *gorm.DB) error {
	now := time.Now().Local()
	if u.CreatedAt.IsZero() {
		u.CreatedAt = now
	}
	if u.UpdatedAt.IsZero() {
		u.UpdatedAt = now
	}
	return nil
}

// BeforeUpdate hook - update timestamp
func (u *UserModel) BeforeUpdate(tx *gorm.DB) error {
	u.UpdatedAt = time.Now().Local()
	return nil
}
