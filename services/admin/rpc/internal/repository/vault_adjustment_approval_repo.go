package repository

import (
	"context"
	"fmt"
	"strings"

	"internalwallet/services/admin/rpc/internal/model"

	"gorm.io/gorm"
)

type VaultAdjustmentApprovalRepository interface {
	WithTx(tx *gorm.DB) VaultAdjustmentApprovalRepository
	GetDB() *gorm.DB

	Create(ctx context.Context, m *model.VaultAdjustmentApprovalModel) error
	ExistsByAdjustmentAndAdmin(ctx context.Context, adjustmentID, adminID int64) (bool, error)
	CountByAdjustmentAndAction(ctx context.Context, adjustmentID int64, action string) (int64, error)
}

type vaultAdjustmentApprovalRepo struct {
	db *gorm.DB
}

func NewVaultAdjustmentApprovalRepository(db *gorm.DB) VaultAdjustmentApprovalRepository {
	return &vaultAdjustmentApprovalRepo{db: db}
}
func (r *vaultAdjustmentApprovalRepo) WithTx(tx *gorm.DB) VaultAdjustmentApprovalRepository {
	return &vaultAdjustmentApprovalRepo{db: tx}
}
func (r *vaultAdjustmentApprovalRepo) GetDB() *gorm.DB { return r.db }

func (r *vaultAdjustmentApprovalRepo) Create(ctx context.Context, m *model.VaultAdjustmentApprovalModel) error {
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *vaultAdjustmentApprovalRepo) ExistsByAdjustmentAndAdmin(ctx context.Context, adjustmentID, adminID int64) (bool, error) {
	if adjustmentID <= 0 || adminID <= 0 {
		return false, nil
	}
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.VaultAdjustmentApprovalModel{}).
		Where("adjustment_id = ? AND admin_id = ?", adjustmentID, adminID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *vaultAdjustmentApprovalRepo) CountByAdjustmentAndAction(ctx context.Context, adjustmentID int64, action string) (int64, error) {
	if adjustmentID <= 0 {
		return 0, nil
	}
	action = strings.TrimSpace(action)
	if action == "" {
		return 0, fmt.Errorf("invalid action")
	}
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.VaultAdjustmentApprovalModel{}).
		Where("adjustment_id = ? AND action = ?", adjustmentID, action).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}
