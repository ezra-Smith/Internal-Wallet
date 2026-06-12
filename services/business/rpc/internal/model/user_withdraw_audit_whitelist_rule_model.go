package model

import commonModel "internalwallet/common/model"

// UserWithdrawAuditWhitelistRuleModel 用户提现免审白名单规则（按地址/代币/额度USDT）。
// - Strict match on chain_code
// - limit_usdt: 0 means unlimited
type UserWithdrawAuditWhitelistRuleModel struct {
	commonModel.BaseModel

	UserID    int64  `gorm:"column:user_id;type:bigint;not null;index"`
	AssetCode string `gorm:"column:asset_code;type:varchar(32);not null;default:'';index"`
	ChainCode string `gorm:"column:chain_code;type:varchar(32);not null;default:'';index"`
	Address   string `gorm:"column:address;type:varchar(128);not null;default:''"`

	WalletName string `gorm:"column:wallet_name;type:varchar(100);not null;default:''"`
	WalletIcon string `gorm:"column:wallet_icon;type:mediumtext"`

	LimitUSDT string `gorm:"column:limit_usdt;type:decimal(36,8);not null;default:0"`
	Enabled   bool   `gorm:"column:enabled;type:tinyint(1);not null;default:1;index"`

	Reason          string `gorm:"column:reason;type:varchar(255);not null;default:''"`
	OperatorAdminID int64  `gorm:"column:operator_admin_id;type:bigint;not null;default:0;index"`

	// Source: admin | user
	Source         string `gorm:"column:source;type:varchar(16);not null;default:'admin';index"`
	OperatorUserID int64  `gorm:"column:operator_user_id;type:bigint;not null;default:0;index"`
}

func (UserWithdrawAuditWhitelistRuleModel) TableName() string {
	return "user_withdraw_audit_whitelist_rules"
}
