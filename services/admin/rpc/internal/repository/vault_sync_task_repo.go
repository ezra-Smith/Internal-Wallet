package repository

import (
	"context"
	"fmt"

	"internalwallet/services/admin/rpc/internal/model"

	"gorm.io/gorm"
)

type VaultSyncTaskRepository interface {
	WithTx(tx *gorm.DB) VaultSyncTaskRepository
	GetDB() *gorm.DB

	Create(ctx context.Context, m *model.VaultSyncTaskModel) error
	UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error
}

type vaultSyncTaskRepo struct {
	db *gorm.DB
}

func NewVaultSyncTaskRepository(db *gorm.DB) VaultSyncTaskRepository {
	return &vaultSyncTaskRepo{db: db}
}
func (r *vaultSyncTaskRepo) WithTx(tx *gorm.DB) VaultSyncTaskRepository {
	return &vaultSyncTaskRepo{db: tx}
}
func (r *vaultSyncTaskRepo) GetDB() *gorm.DB { return r.db }

func (r *vaultSyncTaskRepo) Create(ctx context.Context, m *model.VaultSyncTaskModel) error {
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *vaultSyncTaskRepo) UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error {
	if id <= 0 {
		return fmt.Errorf("sync task not found")
	}
	res := r.db.WithContext(ctx).
		Model(&model.VaultSyncTaskModel{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(fields)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("sync task not found")
	}
	return nil
}
