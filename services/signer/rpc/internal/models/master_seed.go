package models

import (
	"database/sql/driver"
	"encoding/json"
	"internalwallet/common/model"
	"time"
)

// MasterSeed Master Seed配置模型
type MasterSeed struct {
	model.BaseModel
	SeedID          string     `gorm:"column:seed_id" json:"seed_id"`
	SeedName        string     `gorm:"column:seed_name" json:"seed_name"`
	SeedEncrypted   []byte     `gorm:"column:seed_encrypted" json:"-"` // 不返回给API
	SeedHash        string     `gorm:"column:seed_hash" json:"seed_hash"`
	EncryptionSalt  []byte     `gorm:"column:encryption_salt" json:"-"` // 不返回给API
	EncryptionIV    []byte     `gorm:"column:encryption_iv" json:"-"`   // 不返回给API
	KeyVersion      int        `gorm:"column:key_version" json:"key_version"`
	SupportedChains ChainList  `gorm:"column:supported_chains;type:json" json:"supported_chains"`
	Temperature     int        `gorm:"column:temperature" json:"temperature"`
	Status          int        `gorm:"column:status" json:"status"`
	RequireApproval bool       `gorm:"column:require_approval" json:"require_approval"`
	MaxDailyAmount  *string    `gorm:"column:max_daily_amount" json:"max_daily_amount,omitempty"`
	TotalSignatures int64      `gorm:"column:total_signatures" json:"total_signatures"`
	LastSignatureAt *time.Time `gorm:"column:last_signature_at" json:"last_signature_at,omitempty"`
	TeeEnabled      bool       `gorm:"column:tee_enabled" json:"tee_enabled"`
	TeeSealedSeed   []byte     `gorm:"column:tee_sealed_seed" json:"-"`
	TeeMeasurement  *string    `gorm:"column:tee_measurement" json:"tee_measurement,omitempty"`
	Description     *string    `gorm:"column:description" json:"description,omitempty"`
}

// TableName 指定表名
func (MasterSeed) TableName() string {
	return "master_seeds"
}

// ChainList JSON数组类型，用于存储支持的链列表
type ChainList []string

// Value 实现 driver.Valuer 接口
func (c ChainList) Value() (driver.Value, error) {
	if c == nil {
		return nil, nil
	}
	return json.Marshal(c)
}

// Scan 实现 sql.Scanner 接口
func (c *ChainList) Scan(value interface{}) error {
	if value == nil {
		*c = nil
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}

	return json.Unmarshal(bytes, c)
}
