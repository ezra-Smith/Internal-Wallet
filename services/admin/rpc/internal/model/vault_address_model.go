package model

import "time"

// VaultAddressModel maps to vault_addresses (external imported wallet addresses).
//
//	存储外部导入的钱包地址
type VaultAddressModel struct {
	ID        int64  `gorm:"column:id;primaryKey;autoIncrement"`
	NetworkID int64  `gorm:"column:network_id;type:bigint;not null;index"`
	Address   string `gorm:"column:address;type:varchar(255);not null"`

	AddressType string  `gorm:"column:address_type;type:varchar(32);not null;default:'active';index"` // active|hot|cold|deposit
	Label       *string `gorm:"column:label;type:varchar(100)"`

	Status         string `gorm:"column:status;type:varchar(20);not null;default:'inactive';index"` // active|inactive|disabled
	IsActiveWallet int8   `gorm:"column:is_active_wallet;type:tinyint(1);not null;default:0"`       // 同一网络只能有一个活跃钱包

	CreatedBy int64 `gorm:"column:created_by;type:bigint;not null;default:0"`

	CreatedAt *time.Time `gorm:"column:created_at;type:timestamp"`
	UpdatedAt *time.Time `gorm:"column:updated_at;type:timestamp"`
	DeletedAt *time.Time `gorm:"column:deleted_at;type:timestamp"`
}

func (VaultAddressModel) TableName() string { return "vault_addresses" }
