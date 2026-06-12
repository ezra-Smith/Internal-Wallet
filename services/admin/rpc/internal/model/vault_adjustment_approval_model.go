package model

import "time"

// VaultAdjustmentApprovalModel maps to vault_adjustment_approvals (immutable review records).
type VaultAdjustmentApprovalModel struct {
	ID           int64      `gorm:"column:id;primaryKey;autoIncrement"`
	AdjustmentID int64      `gorm:"column:adjustment_id;type:bigint;not null;index"`
	AdminID      int64      `gorm:"column:admin_id;type:bigint;not null;index"`
	Action       string     `gorm:"column:action;type:varchar(16);not null;index"` // approve|reject
	Note         *string    `gorm:"column:note;type:text"`
	CreatedAt    *time.Time `gorm:"column:created_at;type:timestamp;index"`
}

func (VaultAdjustmentApprovalModel) TableName() string { return "vault_adjustment_approvals" }
