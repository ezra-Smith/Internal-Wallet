package model

import (
	commonModel "internalwallet/common/model"
)

// 用户交易记录状态常量
const (
	TxRecordStatusPending   = "pending"   // 待处理
	TxRecordStatusSuccess   = "success"   // 成功（旧）
	TxRecordStatusCompleted = "completed" // 已完成
	TxRecordStatusFailed    = "failed"    // 失败
	TxRecordStatusCancelled = "cancelled" // 已取消
	TxRecordStatusRejected  = "rejected"  // 已拒绝（管理员审核拒绝）
)

// UserTransactionRecordModel 映射到 acct_user_transaction_records 表
// 用于 Admin service 创建内部转账交易记录
type UserTransactionRecordModel struct {
	commonModel.BaseModel

	// 用户和交易类型
	UserId int64 `gorm:"column:user_id;type:bigint;not null;index" json:"user_id"`
	Type   int32 `gorm:"column:tx_type;type:int;not null;index;comment:交易类型：1=deposit 2=withdraw（包含内部转账转出方）" json:"type"`

	// 资产信息
	Asset     string `gorm:"column:asset_code;type:varchar(32);not null;index" json:"asset"`           // 资产代码
	ChainCode string `gorm:"column:chain_code;type:varchar(32);not null;default:''" json:"chain_code"` // 链代码

	// 金额信息（使用 DECIMAL 类型，GORM 会自动处理 string 和 decimal 的转换）
	Amount string `gorm:"column:amount_decimal;type:decimal(65,30);not null;default:0" json:"amount"` // 主金额（资产单位）
	Fee    string `gorm:"column:fee_decimal;type:decimal(65,30);not null;default:0" json:"fee"`       // 手续费（资产单位）

	// 状态
	Status string `gorm:"column:status;type:varchar(20);not null;default:'pending';index" json:"status"` // pending/success/failed/cancelled/completed/rejected
	Memo   string `gorm:"column:memo;type:varchar(255);not null;default:''" json:"memo"`                 // 备注/Memo/Tag

	// 地址信息
	FromAddress string `gorm:"column:from_address;type:varchar(255);not null;default:''" json:"from_address"` // 来源地址
	ToAddress   string `gorm:"column:to_address;type:varchar(255);not null;default:''" json:"to_address"`     // 目标地址
	TxHash      string `gorm:"column:tx_hash;type:varchar(255);not null;default:'';index" json:"tx_hash"`     // 链上交易哈希

	// 账本关联（Accounting 服务使用）
	FreezeLedgerTxID  *int64 `gorm:"column:freeze_ledger_tx_id;type:bigint" json:"freeze_ledger_tx_id"`   // 冻结分录tx_id（withdraw）
	SettleLedgerTxID  *int64 `gorm:"column:settle_ledger_tx_id;type:bigint" json:"settle_ledger_tx_id"`   // 扣款结算分录tx_id（withdraw/内部转账转出方）
	ConfirmLedgerTxID *int64 `gorm:"column:confirm_ledger_tx_id;type:bigint" json:"confirm_ledger_tx_id"` // 入账分录tx_id（deposit/内部转账转入方）

	// 业务引用
	BizRef         string `gorm:"column:biz_ref;type:varchar(128);not null;default:''" json:"biz_ref"`                 // 业务引用（如 withdraw_id/deposit_tx:hash/internal_transfer:order_id）
	IdempotencyKey string `gorm:"column:idempotency_key;type:varchar(128);not null;default:''" json:"idempotency_key"` // 幂等键
}

func (UserTransactionRecordModel) TableName() string {
	return "acct_user_transaction_records"
}
