package model

import (
	commonModel "internalwallet/common/model"
	"time"
)

// SwapSvcTransactionModel maps to swap_svc_transactions.
type SwapSvcTransactionModel struct {
	commonModel.BaseModel
	ApiKeyID      int64  `gorm:"column:api_key_id;type:bigint;not null;index"`
	ProjectName   string `gorm:"column:project_name;type:varchar(128);not null;default:'';index"`
	WalletAddress string `gorm:"column:wallet_address;type:varchar(64);not null;index"`
	ChainID       int64  `gorm:"column:chain_id;type:bigint;not null;index"`
	Provider      string `gorm:"column:provider;type:varchar(32);not null;default:'1inch'"`
	FromToken     string `gorm:"column:from_token;type:varchar(64);not null;default:''"`
	ToToken       string `gorm:"column:to_token;type:varchar(64);not null;default:''"`
	FromAmount    string `gorm:"column:from_amount;type:varchar(100);not null;default:'0'"`
	ToAmount      string `gorm:"column:to_amount;type:varchar(100);not null;default:'0'"`
	// Unsigned transaction fields (from ExecuteSwap)
	TxFrom          string `gorm:"column:tx_from;type:varchar(64);not null;default:''"`
	TxTo            string `gorm:"column:tx_to;type:varchar(64);not null;default:''"`
	TxData          string `gorm:"column:tx_data;type:text"`
	TxValue         string `gorm:"column:tx_value;type:varchar(100);not null;default:'0'"`
	TxGas           string `gorm:"column:tx_gas;type:varchar(100);not null;default:'0'"`
	TxGasPrice      string `gorm:"column:tx_gas_price;type:varchar(100);not null;default:'0'"`
	TxNonce         string `gorm:"column:tx_nonce;type:varchar(50);not null;default:''"`
	TxSignatureData string `gorm:"column:tx_signature_data;type:text"` // OKX特定：额外的签名数据（JSON数组）
	// Transaction result fields
	TxHash        string     `gorm:"column:tx_hash;type:varchar(128);not null;default:'';index"`
	Status        string     `gorm:"column:status;type:varchar(16);not null;default:'created';index"`
	ErrorCode     string     `gorm:"column:error_code;type:varchar(64);not null;default:''"`
	ErrorMessage  *string    `gorm:"column:error_message;type:text"`
	BroadcastedAt *time.Time `gorm:"column:broadcasted_at;type:datetime"`
}

func (SwapSvcTransactionModel) TableName() string { return "swap_svc_transactions" }
