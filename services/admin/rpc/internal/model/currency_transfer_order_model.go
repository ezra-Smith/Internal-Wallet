package model

import (
	"time"

	commonModel "internalwallet/common/model"
)

// CurrencyTransferOrderModel maps to `currency_transfer_orders`.
// Represents internal transfer orders between users with optional audit.
type CurrencyTransferOrderModel struct {
	commonModel.BaseModel

	FromUserID       int64      `gorm:"column:from_user_id;type:bigint;not null;index" json:"from_user_id"`
	ToUserID         int64      `gorm:"column:to_user_id;type:bigint;not null;index" json:"to_user_id"`
	AssetCode        string     `gorm:"column:asset_code;type:varchar(32);not null;index" json:"asset_code"`
	Amount           string     `gorm:"column:amount;type:decimal(65,30);not null" json:"amount"`
	Fee              string     `gorm:"column:fee;type:decimal(65,30);not null;default:0" json:"fee"`
	Note             *string    `gorm:"column:note;type:varchar(512)" json:"note"`
	CreatedByAdmin   int64      `gorm:"column:created_by_admin_id;type:bigint;not null;default:0" json:"created_by_admin_id"`
	Strategy         string     `gorm:"column:strategy;type:varchar(16);not null" json:"strategy"` // auto|manual_auto
	Status           string     `gorm:"column:status;type:varchar(20);not null;default:pending;index" json:"status"`
	ErrorMessage     *string    `gorm:"column:error_message;type:varchar(512)" json:"error_message"`
	LedgerTxID       *int64     `gorm:"column:ledger_tx_id;type:bigint" json:"ledger_tx_id"`
	FreezeLedgerTxID *int64     `gorm:"column:freeze_ledger_tx_id;type:bigint" json:"freeze_ledger_tx_id"` // 冻结交易ID（待审核时记录）
	AuditAdminID     int64      `gorm:"column:audit_admin_id;type:bigint;not null;default:0" json:"audit_admin_id"`
	AuditNote        *string    `gorm:"column:audit_note;type:text" json:"audit_note"`
	AuditedAt        *time.Time `gorm:"column:audited_at" json:"audited_at"`
	UpdatedBy        int64      `gorm:"column:updated_by;type:bigint;not null;default:0" json:"updated_by"`
}

func (CurrencyTransferOrderModel) TableName() string {
	return "currency_transfer_orders"
}

// Transfer order status constants
const (
	TransferStatusPending    = "pending"
	TransferStatusProcessing = "processing"
	TransferStatusCompleted  = "completed"
	TransferStatusFailed     = "failed"
	TransferStatusCancelled  = "cancelled"
	TransferStatusRejected   = "rejected" // 管理员审核拒绝
)

// Transfer strategy constants
const (
	TransferStrategyAuto       = "auto"
	TransferStrategyManualAuto = "manual_auto"
)
