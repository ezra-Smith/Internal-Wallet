package models

import (
	"time"

	"internalwallet/common/model"
)

type EnergyRentalStatus int32

const (
	EnergyRentalStatusPending   EnergyRentalStatus = 1
	EnergyRentalStatusConfirmed EnergyRentalStatus = 2
	EnergyRentalStatusFailed    EnergyRentalStatus = 3
	EnergyRentalStatusExpired   EnergyRentalStatus = 4
)

// EnergyRentalRecord tracks third-party TRON energy rentals (e.g., iTRX).
type EnergyRentalRecord struct {
	model.BaseModel

	OrderID         string `gorm:"column:order_id;size:64;uniqueIndex:uk_energy_rental_records_order_id;not null" json:"order_id"`
	ReceiverAddress string `gorm:"column:receiver_address;size:100;index:idx_energy_rental_records_receiver_address;not null" json:"receiver_address"`

	EnergyAmount   int64  `gorm:"column:energy_amount;not null" json:"energy_amount"`
	RentalDuration int32  `gorm:"column:rental_duration;not null" json:"rental_duration"`
	PricePerEnergy string `gorm:"column:price_per_energy;size:32;not null" json:"price_per_energy"`
	TotalCost      string `gorm:"column:total_cost;size:64;not null" json:"total_cost"`
	Provider       string `gorm:"column:provider;size:20;not null" json:"provider"`

	Status           EnergyRentalStatus `gorm:"column:status;index:idx_energy_rental_records_status;not null;default:1" json:"status"`
	ProviderResponse []byte             `gorm:"column:provider_response;type:json" json:"provider_response,omitempty"`
	ErrorMessage     *string            `gorm:"column:error_message;type:text" json:"error_message,omitempty"`

	ConfirmedAt *time.Time `gorm:"column:confirmed_at" json:"confirmed_at,omitempty"`
}

func (EnergyRentalRecord) TableName() string {
	return "energy_rental_records"
}
