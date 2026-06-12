package model

import (
	"time"

	commonModel "internalwallet/common/model"

	"gorm.io/gorm"
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

// UserTransactionRecordModel 映射到 acct_user_transaction_records 表（Web2 用户的交易记录）
// 这是 Accounting 服务的读模型，用于 Business 服务展示用户交易记录
type UserTransactionRecordModel struct {
	commonModel.BaseModel

	// 用户和交易类型
	UserId int64 `gorm:"column:user_id;type:bigint;not null;index:idx_autr_user_created_at,priority:1;index:idx_autr_user_type_created_at,priority:1;index:idx_autr_user_asset_created_at,priority:1" json:"user_id"`
	Type   int32 `gorm:"column:tx_type;type:int;not null;index:idx_autr_user_type_created_at,priority:2;comment:交易类型：1=deposit 2=withdraw（包含内部转账转出方）" json:"type"`

	// 资产信息
	Asset     string `gorm:"column:asset_code;type:varchar(32);not null;index:idx_autr_user_asset_created_at,priority:2" json:"asset"` // 资产代码
	ChainCode string `gorm:"column:chain_code;type:varchar(32);not null;default:''" json:"chain_code"`                                 // 链代码

	// 金额信息（使用 DECIMAL 类型，GORM 会自动处理 string 和 decimal 的转换）
	// 注意：代码中使用 string，数据库存储为 DECIMAL，GORM 会自动转换
	Amount string `gorm:"column:amount_decimal;type:decimal(65,30);not null;default:0" json:"amount"` // 主金额（资产单位）
	Fee    string `gorm:"column:fee_decimal;type:decimal(65,30);not null;default:0" json:"fee"`       // 手续费（资产单位）

	// 状态
	Status string `gorm:"column:status;type:varchar(20);not null;default:'pending';index:idx_autr_status" json:"status"` // pending/success/failed/cancelled/completed/rejected
	Memo   string `gorm:"column:memo;type:varchar(255);not null;default:''" json:"memo"`                                 // 备注/Memo/Tag

	// 地址信息
	FromAddress string `gorm:"column:from_address;type:varchar(255);not null;default:''" json:"from_address"`              // 来源地址
	ToAddress   string `gorm:"column:to_address;type:varchar(255);not null;default:''" json:"to_address"`                  // 目标地址
	TxHash      string `gorm:"column:tx_hash;type:varchar(255);not null;default:'';index:idx_autr_tx_hash" json:"tx_hash"` // 链上交易哈希

	// 账本关联（Accounting 服务使用）
	FreezeLedgerTxID  *int64 `gorm:"column:freeze_ledger_tx_id;type:bigint" json:"freeze_ledger_tx_id"`   // 冻结分录tx_id（withdraw）
	SettleLedgerTxID  *int64 `gorm:"column:settle_ledger_tx_id;type:bigint" json:"settle_ledger_tx_id"`   // 扣款结算分录tx_id（withdraw）
	ConfirmLedgerTxID *int64 `gorm:"column:confirm_ledger_tx_id;type:bigint" json:"confirm_ledger_tx_id"` // 入账分录tx_id（deposit）

	// 业务引用
	BizRef         string `gorm:"column:biz_ref;type:varchar(128);not null;default:''" json:"biz_ref"`                 // 业务引用（如 withdraw_id/deposit_tx:hash）
	IdempotencyKey string `gorm:"column:idempotency_key;type:varchar(128);not null;default:''" json:"idempotency_key"` // 幂等键

	// 兼容字段（用于 transaction_confirm_processor.go）
	// 这些字段在数据库表中不存在，但代码中使用了，需要通过 GORM 的忽略标签处理
	Timestamp    int64  `gorm:"-" json:"timestamp"`    // 时间戳（从 created_at 转换）
	Address      string `gorm:"-" json:"address"`      // 地址（从 to_address 或 from_address 获取）
	Network      string `gorm:"-" json:"network"`      // 网络（从 chain_code 获取）
	Counterparty string `gorm:"-" json:"counterparty"` // 对手方地址
	TxId         string `gorm:"-" json:"tx_id"`        // 内部交易ID（从 biz_ref 获取）
}

func (UserTransactionRecordModel) TableName() string {
	return "acct_user_transaction_records"
}

// BeforeCreate 在创建前设置字段映射
func (m *UserTransactionRecordModel) BeforeCreate(tx *gorm.DB) error {
	// 调用基类的 BeforeCreate
	if err := m.BaseModel.BeforeCreate(tx); err != nil {
		return err
	}

	// 将兼容字段映射到数据库字段（从兼容字段 -> 数据库字段）
	if m.Timestamp > 0 && m.CreatedAt.IsZero() {
		m.CreatedAt = time.Unix(m.Timestamp, 0)
	}
	if m.Address != "" {
		if m.Type == 1 { // deposit
			if m.ToAddress == "" {
				m.ToAddress = m.Address
			}
		} else { // withdraw
			if m.FromAddress == "" {
				m.FromAddress = m.Address
			}
		}
	}
	if m.Network != "" && m.ChainCode == "" {
		m.ChainCode = m.Network
	}
	if m.Counterparty != "" {
		if m.Type == 1 { // deposit
			if m.FromAddress == "" {
				m.FromAddress = m.Counterparty
			}
		} else { // withdraw
			if m.ToAddress == "" {
				m.ToAddress = m.Counterparty
			}
		}
	}
	if m.TxId != "" && m.BizRef == "" {
		m.BizRef = m.TxId
	}

	return nil
}

// AfterFind 在查询后设置兼容字段
func (m *UserTransactionRecordModel) AfterFind(tx *gorm.DB) error {
	// 设置兼容字段
	if m.CreatedAt.Unix() > 0 {
		m.Timestamp = m.CreatedAt.Unix()
	}
	if m.Address == "" {
		if m.ToAddress != "" {
			m.Address = m.ToAddress
		} else if m.FromAddress != "" {
			m.Address = m.FromAddress
		}
	}
	if m.Network == "" {
		m.Network = m.ChainCode
	}
	if m.Counterparty == "" {
		if m.Type == 1 { // deposit
			m.Counterparty = m.FromAddress
		} else { // withdraw
			m.Counterparty = m.ToAddress
		}
	}
	if m.TxId == "" && m.BizRef != "" {
		m.TxId = m.BizRef
	}

	return nil
}
