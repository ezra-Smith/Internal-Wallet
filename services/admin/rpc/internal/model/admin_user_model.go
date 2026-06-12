package model

import (
	commonModel "internalwallet/common/model"
	"time"
)

// AdminUserModel 管理员表
// 表结构建议见 deploy/docker/init-db/03-admin.sql
type AdminUserModel struct {
	commonModel.BaseModel

	Username string `gorm:"column:username;type:varchar(128);not null;index"`
	// PasswordHash 使用 bcrypt 存储（不需要单独 salt 字段）
	PasswordHash string `gorm:"column:password_hash;type:varchar(255);not null"`

	Name string `gorm:"column:name;type:varchar(64);not null"`
	Role string `gorm:"column:role;type:varchar(32);not null;index"`
	// active/disabled
	Status string `gorm:"column:status;type:varchar(16);not null;default:'active';index"`

	// TwoFactorSecret stores a Base32 TOTP secret or an encrypted value (enc:...).
	TwoFactorSecret        string     `gorm:"column:two_factor_secret;type:varchar(255);default:''"`
	TwoFactorPendingSecret string     `gorm:"column:two_factor_pending_secret;type:varchar(255);default:''"`
	TwoFactorEnabled       bool       `gorm:"column:two_factor_enabled;type:tinyint(1);not null;default:0"`
	TwoFactorBoundAt       *time.Time `gorm:"column:two_factor_bound_at;type:datetime;default:null"`

	RequirePasswordChange bool `gorm:"column:require_password_change;type:tinyint(1);not null;default:0"`
	TwoFactorRequired     bool `gorm:"column:two_factor_required;type:tinyint(1);not null;default:0"`

	FailedLoginCount int        `gorm:"column:failed_login_count;type:int;not null;default:0"`
	LockUntil        *time.Time `gorm:"column:lock_until;type:datetime;default:null"`

	LastLoginAt       *time.Time `gorm:"column:last_login_at;type:datetime;default:null"`
	LastLoginIP       string     `gorm:"column:last_login_ip;type:varchar(45);default:''"`
	PasswordChangedAt *time.Time `gorm:"column:password_changed_at;type:datetime;default:null"`

	// TokenVersion 用于强制失效全量会话（例如禁用管理员、重置密码等）
	TokenVersion int64 `gorm:"column:token_version;type:bigint;not null;default:1"`

	CreatedBy int64 `gorm:"column:created_by;type:bigint;default:0"`
	UpdatedBy int64 `gorm:"column:updated_by;type:bigint;default:0"`
}

func (AdminUserModel) TableName() string { return "admin_users" }
