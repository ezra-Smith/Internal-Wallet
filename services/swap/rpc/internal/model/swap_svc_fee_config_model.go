package model

import "time"

// SwapSvcFeeConfigModel maps to swap_svc_fee_config.
type SwapSvcFeeConfigModel struct {
	ID            int64     `gorm:"column:id;type:bigint;primaryKey;autoIncrement:false"`
	ChainID       int64     `gorm:"column:chain_id;type:bigint;not null;uniqueIndex:uk_swap_svc_fee_chain_provider,priority:1"`
	Provider      string    `gorm:"column:provider;type:varchar(32);not null;uniqueIndex:uk_swap_svc_fee_chain_provider,priority:2"`
	FeePercentage string    `gorm:"column:fee_percentage;type:decimal(10,6);not null;default:0"`
	MinFee        string    `gorm:"column:min_fee;type:decimal(36,18);not null;default:0"`
	MaxFee        string    `gorm:"column:max_fee;type:decimal(36,18);not null;default:0"`
	CreatedAt     time.Time `gorm:"column:created_at;type:datetime;not null"`
	UpdatedAt     time.Time `gorm:"column:updated_at;type:datetime;not null"`
}

func (SwapSvcFeeConfigModel) TableName() string { return "swap_svc_fee_config" }
