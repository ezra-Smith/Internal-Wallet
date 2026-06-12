package model

import (
	commonModel "internalwallet/common/model"
	"time"
)

// CurrencyWithdrawOrderModel maps to `currency_withdraw_orders`.
type CurrencyWithdrawOrderModel struct {
	commonModel.BaseModel

	UserID    int64  `gorm:"column:user_id;type:bigint;not null;index"`
	AssetCode string `gorm:"column:asset_code;type:varchar(32);not null;index"`
	ChainCode string `gorm:"column:chain_code;type:varchar(32);not null;index"`

	Amount string `gorm:"column:amount;type:decimal(65,30);not null"`
	Fee    string `gorm:"column:fee;type:decimal(65,30);not null;default:0"`

	FeeRuleSnapshot   []byte  `gorm:"column:fee_rule_snapshot;type:json"`
	FeeRuleSummary    *string `gorm:"column:fee_rule_summary;type:varchar(255);default:null"`
	FeeRuleSource     string  `gorm:"column:fee_rule_source;type:varchar(16);not null;default:'global'"`
	AuditRuleSnapshot []byte  `gorm:"column:audit_rule_snapshot;type:json"`

	FromAddress      string  `gorm:"column:from_address;type:varchar(255);not null;default:''"`
	ToAddress        string  `gorm:"column:to_address;type:varchar(255);not null;default:''"`
	MemoTag          *string `gorm:"column:memo_tag;type:varchar(255);default:null"`
	CreatedByAdminID int64   `gorm:"column:created_by_admin_id;type:bigint;not null;default:0"`

	Strategy string  `gorm:"column:strategy;type:varchar(16);not null"`                       // auto|manual_auto|manual_manual
	Status   string  `gorm:"column:status;type:varchar(20);not null;default:'pending';index"` // pending|processing|completed|failed|cancelled
	TxHash   *string `gorm:"column:tx_hash;type:varchar(128);default:null;index"`

	ErrorMessage *string `gorm:"column:error_message;type:varchar(512);default:null"`

	AuditAdminID int64      `gorm:"column:audit_admin_id;type:bigint;not null;default:0;index"`
	AuditNote    *string    `gorm:"column:audit_note;type:text"`
	AuditedAt    *time.Time `gorm:"column:audited_at;type:datetime;default:null"`

	TransferAdminID int64      `gorm:"column:transfer_admin_id;type:bigint;not null;default:0;index"`
	TransferNote    *string    `gorm:"column:transfer_note;type:text"`
	TransferredAt   *time.Time `gorm:"column:transferred_at;type:datetime;default:null"`

	UpdatedBy int64 `gorm:"column:updated_by;type:bigint;not null;default:0"`
}

func (CurrencyWithdrawOrderModel) TableName() string { return "currency_withdraw_orders" }
