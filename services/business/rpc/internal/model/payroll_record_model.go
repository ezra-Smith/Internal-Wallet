package model

import (
	commonModel "internalwallet/common/model"
	"time"
)

type PayrollRecordModel struct {
	commonModel.BaseModel
	PayrollId      string    `gorm:"column:payroll_id;type:varchar(32);not null;uniqueIndex"`
	BatchId        string    `gorm:"column:batch_id;type:varchar(32);default:'';index"`
	SenderUid      string    `gorm:"column:sender_uid;type:varchar(32);default:'';index"`
	SenderUserId   int64     `gorm:"column:sender_user_id;type:bigint;default:0"`
	ReceiverUid    string    `gorm:"column:receiver_uid;type:varchar(32);default:'';index"`
	ReceiverUserId int64     `gorm:"column:receiver_user_id;type:bigint;default:0"`
	Asset          string    `gorm:"column:asset;type:varchar(20);default:''"`
	Chain          string    `gorm:"column:chain;type:varchar(10);default:''"`
	Amount         string    `gorm:"column:amount;type:decimal(36,18);default:'0'"`
	AmountUsdt     string    `gorm:"column:amount_usdt;type:decimal(36,18);default:'0'"`
	Memo           string    `gorm:"column:memo;type:varchar(500);default:''"`
	PayrollType    int32     `gorm:"column:payroll_type;type:tinyint;default:0"`
	Status         int32     `gorm:"column:status;type:tinyint;default:1;index"`
	TxId           string    `gorm:"column:tx_id;type:varchar(64);default:''"`
	ErrorMessage   string    `gorm:"column:error_message;type:text"`
	ProcessedAt    time.Time `gorm:"column:processed_at;type:datetime"`
	CreatedAt      time.Time `gorm:"column:created_at;type:datetime"`
	UpdatedAt      time.Time `gorm:"column:updated_at;type:datetime"`
}

func (PayrollRecordModel) TableName() string { return "payroll_records" }
