package models

import (
	"internalwallet/common/model"
)

// HDWalletConfig HD钱包链配置
type HDWalletConfig struct {
	model.BaseModel
	Chain          string `gorm:"column:chain;size:20;not null;uniqueIndex:uk_chain" json:"chain"`
	CoinType       int    `gorm:"column:coin_type;not null" json:"coin_type"`
	Account        int    `gorm:"column:account;default:0" json:"account"`
	DerivationRule string `gorm:"column:derivation_rule;size:128" json:"derivation_rule"`
	AddressFormat  string `gorm:"column:address_format;size:50" json:"address_format"`
	Status         int    `gorm:"column:status;default:1" json:"status"`
}

// TableName 指定表名
func (HDWalletConfig) TableName() string {
	return "hd_wallet_config"
}
