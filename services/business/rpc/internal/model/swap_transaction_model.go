package model

import (
	commonModel "internalwallet/common/model"
	"time"
)

type SwapTransactionModel struct {
	commonModel.BaseModel
	SwapId            string    `gorm:"column:swap_id;type:varchar(32);not null;uniqueIndex"`
	UserId            int64     `gorm:"column:user_id;type:bigint;not null;index"`
	FromAsset         string    `gorm:"column:from_asset;type:varchar(20);default:''"`
	FromChain         string    `gorm:"column:from_chain;type:varchar(10);default:''"`
	FromAmount        string    `gorm:"column:from_amount;type:decimal(36,18);default:'0'"`
	ToAsset           string    `gorm:"column:to_asset;type:varchar(20);default:''"`
	ToChain           string    `gorm:"column:to_chain;type:varchar(10);default:''"`
	ToAmount          string    `gorm:"column:to_amount;type:decimal(36,18);default:'0'"`
	ToAmountActual    string    `gorm:"column:to_amount_actual;type:decimal(36,18);default:'0'"`
	ExchangeRate      string    `gorm:"column:exchange_rate;type:decimal(36,18);default:'0'"`
	SlippageTolerance string    `gorm:"column:slippage_tolerance;type:decimal(5,4);default:'0'"`
	SlippageActual    string    `gorm:"column:slippage_actual;type:decimal(5,4);default:'0'"`
	FeeAmount         string    `gorm:"column:fee_amount;type:decimal(36,18);default:'0'"`
	FeeAsset          string    `gorm:"column:fee_asset;type:varchar(20);default:''"`
	FeeRate           string    `gorm:"column:fee_rate;type:decimal(8,6);default:'0'"`
	DexProvider       string    `gorm:"column:dex_provider;type:varchar(50);default:''"`
	DexRoute          string    `gorm:"column:dex_route;type:json"`
	QuoteId           string    `gorm:"column:quote_id;type:varchar(64);default:''"`
	QuoteExpiresAt    time.Time `gorm:"column:quote_expires_at;type:datetime"`
	TxHash            string    `gorm:"column:tx_hash;type:varchar(100);default:'';index"`
	TxStatus          int32     `gorm:"column:tx_status;type:tinyint;default:0"`
	Confirmations     int       `gorm:"column:confirmations;type:int;default:0"`
	Status            int32     `gorm:"column:status;type:tinyint;default:0;index"`
	ErrorCode         string    `gorm:"column:error_code;type:varchar(20);default:''"`
	ErrorMessage      string    `gorm:"column:error_message;type:text"`
	PriceImpact       string    `gorm:"column:price_impact;type:decimal(8,6);default:'0'"`
	MinReceiveAmount  string    `gorm:"column:min_receive_amount;type:decimal(36,18);default:'0'"`
	CreatedAt         time.Time `gorm:"column:created_at;type:datetime"`
	QuotedAt          time.Time `gorm:"column:quoted_at;type:datetime"`
	ConfirmedAt       time.Time `gorm:"column:confirmed_at;type:datetime"`
	CompletedAt       time.Time `gorm:"column:completed_at;type:datetime"`
	UpdatedAt         time.Time `gorm:"column:updated_at;type:datetime"`
}

func (SwapTransactionModel) TableName() string { return "swap_transactions" }
