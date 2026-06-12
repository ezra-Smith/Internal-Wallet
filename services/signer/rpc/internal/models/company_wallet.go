package models

import (
	"internalwallet/common/model"
)

// CompanyWallet 公司钱包地址模型
// 存储公司的热钱包、冷钱包、归集地址等
type CompanyWallet struct {
	model.BaseModel
	AddressIndex     int     `gorm:"column:address_index;not null" json:"address_index"`                             // 地址索引 (1-100)
	AddressType      string  `gorm:"column:address_type;type:varchar(30);not null" json:"address_type"`              // hot_primary, cold_primary, etc.
	Chain            string  `gorm:"column:chain;type:varchar(10);not null" json:"chain"`                            // ETH, BSC, TRN
	Address          string  `gorm:"column:address;type:varchar(100);not null" json:"address"`                       // 钱包地址
	DerivationPath   string  `gorm:"column:derivation_path;type:varchar(100);not null" json:"derivation_path"`       // HD派生路径
	SeedID           string  `gorm:"column:seed_id;type:varchar(50);not null" json:"seed_id"`                        // hot_wallet_main 或 cold_wallet_main
	Temperature      int8    `gorm:"column:temperature;not null" json:"temperature"`                                 // 1=热钱包, 2=冷钱包
	Status           int8    `gorm:"column:status;default:1" json:"status"`                                          // 1=启用, 0=禁用
	IsDefault        int8    `gorm:"column:is_default;default:0" json:"is_default"`                                  // 是否为该链的默认钱包（1=是, 0=否）
	BalanceThreshold float64 `gorm:"column:balance_threshold;type:decimal(20,8);default:0" json:"balance_threshold"` // 余额阈值
	Remark           string  `gorm:"column:remark;type:varchar(500)" json:"remark"`                                  // 备注
}

// TableName 指定表名
func (CompanyWallet) TableName() string {
	return "company_wallets"
}
