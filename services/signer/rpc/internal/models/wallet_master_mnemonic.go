package models

import (
	"internalwallet/common/model"
	"time"
)

// WalletMasterMnemonic 主助记词存储（初始化向导）
// 注意：不保存明文助记词与解锁密码，只保存加密助记词与 bcrypt 校验值。
type WalletMasterMnemonic struct {
	model.BaseModel

	SeedID string `gorm:"column:seed_id" json:"seed_id"`

	MnemonicEncrypted []byte `gorm:"column:mnemonic_encrypted" json:"-"`
	MnemonicSalt      []byte `gorm:"column:mnemonic_salt" json:"-"`
	MnemonicIV        []byte `gorm:"column:mnemonic_iv" json:"-"`

	UnlockPasswordBcrypt string `gorm:"column:unlock_password_bcrypt" json:"-"`

	BackupConfirmedAt *time.Time `gorm:"column:backup_confirmed_at" json:"backup_confirmed_at,omitempty"`

	CreatedByAdminID int64 `gorm:"column:created_by_admin_id" json:"created_by_admin_id"`

	WordCount int `gorm:"column:word_count" json:"word_count"`
	Version   int `gorm:"column:version" json:"version"`
}

func (WalletMasterMnemonic) TableName() string {
	return "wallet_master_mnemonics"
}
