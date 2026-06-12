package models

import "internalwallet/common/model"

// PubkeyExportLog 公钥导出日志模型
type PubkeyExportLog struct {
	model.BaseModel
	MasterSeedID   int64   `gorm:"column:master_seed_id" json:"master_seed_id"`
	Chain          string  `gorm:"column:chain" json:"chain"`
	DerivationPath string  `gorm:"column:derivation_path" json:"derivation_path"`
	Requester      *string `gorm:"column:requester" json:"requester,omitempty"`
	RequestIP      *string `gorm:"column:request_ip" json:"request_ip,omitempty"`
	IsBatch        bool    `gorm:"column:is_batch" json:"is_batch"`
	BatchCount     int     `gorm:"column:batch_count" json:"batch_count"`
}

// TableName 指定表名
func (PubkeyExportLog) TableName() string {
	return "pubkey_export_logs"
}
