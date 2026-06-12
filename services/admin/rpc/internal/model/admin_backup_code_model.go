package model

import (
	commonModel "internalwallet/common/model"
	"time"
)

// AdminBackupCodeModel stores hashed one-time backup codes for 2FA recovery.
// Codes are stored as hashes (with server-side pepper) and should be treated as secrets.
type AdminBackupCodeModel struct {
	commonModel.BaseModel

	AdminID  int64      `gorm:"column:admin_id;type:bigint;not null;index"`
	CodeHash string     `gorm:"column:code_hash;type:varchar(128);not null;index"`
	UsedAt   *time.Time `gorm:"column:used_at;type:datetime;default:null"`

	CreatedBy int64 `gorm:"column:created_by;type:bigint;default:0"`
	UpdatedBy int64 `gorm:"column:updated_by;type:bigint;default:0"`
}

func (AdminBackupCodeModel) TableName() string { return "admin_backup_codes" }
