package model

import (
	commonModel "internalwallet/common/model"
	"time"
)

type WithdrawalAuditModel struct {
	commonModel.BaseModel
	WithdrawId       string    `gorm:"column:withdraw_id;type:varchar(32);not null;uniqueIndex"`
	UserId           int64     `gorm:"column:user_id;type:bigint;not null;index"`
	Asset            string    `gorm:"column:asset;type:varchar(20);default:''"`
	Chain            string    `gorm:"column:chain;type:varchar(10);default:''"`
	Amount           string    `gorm:"column:amount;type:decimal(36,18);default:'0'"`
	AmountUsdt       string    `gorm:"column:amount_usdt;type:decimal(36,18);default:'0'"`
	ToAddress        string    `gorm:"column:to_address;type:varchar(100);default:''"`
	AuditType        int32     `gorm:"column:audit_type;type:tinyint;default:0;index"`
	AuditStatus      int32     `gorm:"column:audit_status;type:tinyint;default:1;index"`
	TriggerReason    string    `gorm:"column:trigger_reason;type:varchar(255);default:''"`
	RiskScore        int       `gorm:"column:risk_score;type:int;default:0"`
	RiskFactors      string    `gorm:"column:risk_factors;type:json"`
	AuditorId        int64     `gorm:"column:auditor_id;type:bigint;default:0"`
	AuditComment     string    `gorm:"column:audit_comment;type:text"`
	AuditStartedAt   time.Time `gorm:"column:audit_started_at;type:datetime"`
	AuditCompletedAt time.Time `gorm:"column:audit_completed_at;type:datetime"`
	TimeoutAt        time.Time `gorm:"column:timeout_at;type:datetime"`
	CreatedAt        time.Time `gorm:"column:created_at;type:datetime"`
	UpdatedAt        time.Time `gorm:"column:updated_at;type:datetime"`
}

func (WithdrawalAuditModel) TableName() string { return "withdrawal_audits" }
