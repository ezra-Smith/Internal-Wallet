package models

import (
	"internalwallet/common/model"
)

// AddressGenerationLog 地址生成日志（只增不改，仅记录生成事实）
// 用于审计和追溯，不包含业务状态和统计字段
type AddressGenerationLog struct {
	model.BaseModel
	UserID           int64  `gorm:"column:user_id;not null;index:idx_user_chain" json:"user_id"`
	Chain            string `gorm:"column:chain;size:20;not null;index:idx_user_chain" json:"chain"`
	Address          string `gorm:"column:address;size:128;not null;uniqueIndex:uk_address" json:"address"`
	DerivationPath   string `gorm:"column:derivation_path;size:128;not null" json:"derivation_path"`
	AddressIndex     int    `gorm:"column:address_index;not null" json:"address_index"`
	PublicKey        string `gorm:"column:public_key;size:256" json:"public_key,omitempty"`
	CompressedPubkey string `gorm:"column:compressed_pubkey;size:128" json:"compressed_pubkey,omitempty"`
	MasterSeedID     *int64 `gorm:"column:master_seed_id;index:idx_master_seed_id" json:"master_seed_id,omitempty"`
	GeneratedBy      string `gorm:"column:generated_by;size:50;default:'system';comment:生成来源: admin/business/system" json:"generated_by"`
}

// TableName 指定表名
func (AddressGenerationLog) TableName() string {
	return "address_generation_logs"
}
