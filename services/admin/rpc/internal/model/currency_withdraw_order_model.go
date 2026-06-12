package model

import (
	commonModel "internalwallet/common/model"
	"time"
)

// CurrencyWithdrawOrderModel maps to `currency_withdraw_orders`.
type CurrencyWithdrawOrderModel struct {
	commonModel.BaseModel

	UserID    int64  `gorm:"column:user_id;type:bigint;not null;index" json:"user_id"`
	AssetCode string `gorm:"column:asset_code;type:varchar(32);not null;index" json:"asset_code"`
	ChainCode string `gorm:"column:chain_code;type:varchar(32);not null;index" json:"chain_code"`

	Amount string `gorm:"column:amount;type:decimal(65,30);not null" json:"amount"`
	Fee    string `gorm:"column:fee;type:decimal(65,30);not null;default:0" json:"fee"`

	FeeRuleSnapshot   []byte  `gorm:"column:fee_rule_snapshot;type:json" json:"fee_rule_snapshot"`
	FeeRuleSummary    *string `gorm:"column:fee_rule_summary;type:varchar(255);default:null" json:"fee_rule_summary"`
	FeeRuleSource     string  `gorm:"column:fee_rule_source;type:varchar(16);not null;default:'global'" json:"fee_rule_source"`
	AuditRuleSnapshot []byte  `gorm:"column:audit_rule_snapshot;type:json" json:"audit_rule_snapshot"`

	FromAddress      string  `gorm:"column:from_address;type:varchar(255);not null;default:''" json:"from_address"`
	ToAddress        string  `gorm:"column:to_address;type:varchar(255);not null;default:''" json:"to_address"`
	MemoTag          *string `gorm:"column:memo_tag;type:varchar(255);default:null" json:"memo_tag"`
	CreatedByAdminID int64   `gorm:"column:created_by_admin_id;type:bigint;not null;default:0" json:"created_by_admin_id"`

	Strategy string  `gorm:"column:strategy;type:varchar(16);not null" json:"strategy"` // auto|manual_auto|manual_manual
	Status   string  `gorm:"column:status;type:varchar(20);not null;default:'pending';index" json:"status"`
	TxHash   *string `gorm:"column:tx_hash;type:varchar(128);default:null;index" json:"tx_hash"`

	ErrorMessage *string `gorm:"column:error_message;type:varchar(512);default:null" json:"error_message"`

	AuditAdminID int64      `gorm:"column:audit_admin_id;type:bigint;not null;default:0;index" json:"audit_admin_id"`
	AuditNote    *string    `gorm:"column:audit_note;type:text" json:"audit_note"`
	AuditedAt    *time.Time `gorm:"column:audited_at;type:datetime;default:null" json:"audited_at"`

	TransferAdminID int64      `gorm:"column:transfer_admin_id;type:bigint;not null;default:0;index" json:"transfer_admin_id"`
	TransferNote    *string    `gorm:"column:transfer_note;type:text" json:"transfer_note"`
	TransferredAt   *time.Time `gorm:"column:transferred_at;type:datetime;default:null" json:"transferred_at"`

	UpdatedBy int64 `gorm:"column:updated_by;type:bigint;not null;default:0" json:"updated_by"`
}

func (CurrencyWithdrawOrderModel) TableName() string { return "currency_withdraw_orders" }
