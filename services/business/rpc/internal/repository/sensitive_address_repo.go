package repository

import (
	"context"
	"strings"

	"gorm.io/gorm"
)

type SensitiveAddressInfo struct {
	RiskLevel     string
	MonitorStatus string
	Reason        string
}

type SensitiveAddressRepository interface {
	FindActive(ctx context.Context, network string, address string) (*SensitiveAddressInfo, error)
}

type sensitiveAddressRepo struct {
	db *gorm.DB
}

func NewSensitiveAddressRepository(db *gorm.DB) SensitiveAddressRepository {
	return &sensitiveAddressRepo{db: db}
}

func (r *sensitiveAddressRepo) FindActive(ctx context.Context, network string, address string) (*SensitiveAddressInfo, error) {
	network = strings.ToLower(strings.TrimSpace(network))
	address = strings.TrimSpace(address)
	if network == "" || address == "" {
		return nil, nil
	}

	var row struct {
		RiskLevel     string  `gorm:"column:risk_level"`
		MonitorStatus string  `gorm:"column:monitor_status"`
		Reason        *string `gorm:"column:reason"`
	}
	tx := r.db.WithContext(ctx).
		Table("blacklist_addresses").
		Select("risk_level, monitor_status, reason").
		Where("deleted_at IS NULL AND network = ? AND address = ?", network, address).
		Limit(1).
		Find(&row)
	if tx.Error != nil {
		return nil, tx.Error
	}
	if tx.RowsAffected == 0 {
		return nil, nil
	}

	reason := ""
	if row.Reason != nil {
		reason = strings.TrimSpace(*row.Reason)
	}
	return &SensitiveAddressInfo{
		RiskLevel:     strings.TrimSpace(row.RiskLevel),
		MonitorStatus: strings.TrimSpace(row.MonitorStatus),
		Reason:        reason,
	}, nil
}
