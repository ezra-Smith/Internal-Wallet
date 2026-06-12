package models

import (
	"internalwallet/common/model"
	"time"
)

// MnemonicBackup 用户助记词备份记录
// 用于助记词验证挑战功能
type MnemonicBackup struct {
	model.BaseModel
	UserID             int64      `gorm:"column:user_id;uniqueIndex:idx_user_seed" json:"user_id"`
	SeedID             string     `gorm:"column:seed_id;uniqueIndex:idx_user_seed" json:"seed_id"`
	MnemonicEncrypted  []byte     `gorm:"column:mnemonic_encrypted" json:"-"` // 加密的助记词，不返回
	EncryptionSalt     []byte     `gorm:"column:encryption_salt" json:"-"`
	EncryptionIV       []byte     `gorm:"column:encryption_iv" json:"-"`
	MnemonicHash       string     `gorm:"column:mnemonic_hash" json:"mnemonic_hash"` // 助记词哈希（用于完整性校验）
	WordCount          int        `gorm:"column:word_count" json:"word_count"`       // 12或24
	BackupConfirmed    bool       `gorm:"column:backup_confirmed" json:"backup_confirmed"`
	BackupConfirmedAt  *time.Time `gorm:"column:backup_confirmed_at" json:"backup_confirmed_at,omitempty"`
	LastChallengedAt   *time.Time `gorm:"column:last_challenged_at" json:"last_challenged_at,omitempty"`
	ChallengeCount     int        `gorm:"column:challenge_count" json:"challenge_count"`             // 总挑战次数
	SuccessfulVerifies int        `gorm:"column:successful_verifies" json:"successful_verifies"`     // 成功验证次数
	FailedVerifies     int        `gorm:"column:failed_verifies" json:"failed_verifies"`             // 失败验证次数
	Status             int        `gorm:"column:status;default:1" json:"status"`                     // 1=正常 2=已删除
	BackupSource       string     `gorm:"column:backup_source" json:"backup_source,omitempty"`       // init_seed/import_mnemonic
	ClientInfo         *string    `gorm:"column:client_info;type:text" json:"client_info,omitempty"` // 客户端信息
}

// TableName 指定表名
func (MnemonicBackup) TableName() string {
	return "mnemonic_backups"
}
