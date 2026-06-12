package model

import (
	commonModel "internalwallet/common/model"
	"time"
)

type TransferAuditModel struct {
	commonModel.BaseModel
	TransferId       string    `gorm:"column:transfer_id;type:varchar(32);not null;uniqueIndex"`
	FromUserId       int64     `gorm:"column:from_user_id;type:bigint;not null;index"`
	ToUserId         int64     `gorm:"column:to_user_id;type:bigint;not null;index"`
	Asset            string    `gorm:"column:asset;type:varchar(20);default:''"`
	Amount           string    `gorm:"column:amount;type:decimal(36,18);default:'0'"`
	AmountUsdt       string    `gorm:"column:amount_usdt;type:decimal(36,18);default:'0'"`
	Memo             string    `gorm:"column:memo;type:text"`
	AuditType        int32     `gorm:"column:audit_type;type:tinyint;default:0;index"`
	AuditStatus      int32     `gorm:"column:audit_status;type:tinyint;default:1;index"`
	TriggerReason    string    `gorm:"column:trigger_reason;type:varchar(255);default:''"`
	RiskScore        int       `gorm:"column:risk_score;type:int;default:0"`
	AuditorId        int64     `gorm:"column:auditor_id;type:bigint;default:0"`
	AuditComment     string    `gorm:"column:audit_comment;type:text"`
	AuditCompletedAt time.Time `gorm:"column:audit_completed_at;type:datetime"`
	CreatedAt        time.Time `gorm:"column:created_at;type:datetime"`
	UpdatedAt        time.Time `gorm:"column:updated_at;type:datetime"`
}

func (TransferAuditModel) TableName() string { return "transfer_audits" }
