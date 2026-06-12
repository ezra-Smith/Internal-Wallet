package repository

import (
	"context"
	"fmt"
	"strings"

	"internalwallet/services/admin/rpc/internal/model"

	"gorm.io/gorm"
)

type VaultThresholdRepository interface {
	WithTx(tx *gorm.DB) VaultThresholdRepository
	GetDB() *gorm.DB

	ListByNetworkID(ctx context.Context, networkID int64) ([]*model.VaultThresholdModel, error)
	Upsert(ctx context.Context, m *model.VaultThresholdModel) error
}

type vaultThresholdRepo struct {
	db *gorm.DB
}

func NewVaultThresholdRepository(db *gorm.DB) VaultThresholdRepository {
	return &vaultThresholdRepo{db: db}
}
func (r *vaultThresholdRepo) WithTx(tx *gorm.DB) VaultThresholdRepository {
	return &vaultThresholdRepo{db: tx}
}
func (r *vaultThresholdRepo) GetDB() *gorm.DB { return r.db }

func (r *vaultThresholdRepo) ListByNetworkID(ctx context.Context, networkID int64) ([]*model.VaultThresholdModel, error) {
	if networkID <= 0 {
		return nil, nil
	}
	var items []*model.VaultThresholdModel
	err := r.db.WithContext(ctx).
		Where("network_id = ? AND deleted_at IS NULL", networkID).
		Order("id ASC").
		Find(&items).Error
	return items, err
}

func (r *vaultThresholdRepo) Upsert(ctx context.Context, m *model.VaultThresholdModel) error {
	if m == nil || m.NetworkID <= 0 || strings.TrimSpace(m.Currency) == "" {
		return fmt.Errorf("invalid threshold")
	}
	m.Currency = strings.TrimSpace(m.Currency)
	updates := map[string]interface{}{
		"currency":               m.Currency,
		"threshold_low":          m.ThresholdLow,
		"threshold_low_raw":      m.ThresholdLowRaw,
		"threshold_critical":     m.ThresholdCritical,
		"threshold_critical_raw": m.ThresholdCriticalRaw,
		"notifications":          m.Notifications,
		"updated_by":             m.UpdatedBy,
		"updated_at":             m.UpdatedAt,
	}
	res := r.db.WithContext(ctx).
		Model(&model.VaultThresholdModel{}).
		Where("network_id = ? AND currency = ? AND deleted_at IS NULL", m.NetworkID, m.Currency).
		Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected > 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).Create(m).Error; err != nil {
		res := r.db.WithContext(ctx).
			Model(&model.VaultThresholdModel{}).
			Where("network_id = ? AND currency = ? AND deleted_at IS NULL", m.NetworkID, m.Currency).
			Updates(updates)
		return res.Error
	}
	return nil
}
