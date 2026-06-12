package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"internalwallet/services/admin/rpc/internal/model"

	"gorm.io/gorm"
)

type VaultNetworkRepository interface {
	WithTx(tx *gorm.DB) VaultNetworkRepository
	GetDB() *gorm.DB

	FindByID(ctx context.Context, id int64) (*model.VaultNetworkModel, error)
	FindByChainID(ctx context.Context, chainID int64) (*model.VaultNetworkModel, error)
	List(ctx context.Context, status string) ([]*model.VaultNetworkModel, error)
	UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error
}

type vaultNetworkRepo struct {
	db *gorm.DB
}

func NewVaultNetworkRepository(db *gorm.DB) VaultNetworkRepository { return &vaultNetworkRepo{db: db} }
func (r *vaultNetworkRepo) WithTx(tx *gorm.DB) VaultNetworkRepository {
	return &vaultNetworkRepo{db: tx}
}
func (r *vaultNetworkRepo) GetDB() *gorm.DB { return r.db }

func (r *vaultNetworkRepo) FindByID(ctx context.Context, id int64) (*model.VaultNetworkModel, error) {
	if id == 0 {
		return nil, fmt.Errorf("network not found")
	}
	var m model.VaultNetworkModel
	err := r.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", id).
		First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("network not found")
	}
	return &m, err
}

func (r *vaultNetworkRepo) FindByChainID(ctx context.Context, chainID int64) (*model.VaultNetworkModel, error) {
	if chainID == 0 {
		return nil, fmt.Errorf("network not found")
	}
	var m model.VaultNetworkModel
	err := r.db.WithContext(ctx).
		Where("chain_id = ? AND deleted_at IS NULL", chainID).
		First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("network not found")
	}
	return &m, err
}

func (r *vaultNetworkRepo) List(ctx context.Context, status string) ([]*model.VaultNetworkModel, error) {
	q := r.db.WithContext(ctx).Model(&model.VaultNetworkModel{}).Where("deleted_at IS NULL")
	if strings.TrimSpace(status) != "" {
		q = q.Where("status = ?", strings.TrimSpace(status))
	}
	var items []*model.VaultNetworkModel
	err := q.Order("chain_id ASC").Find(&items).Error
	return items, err
}

func (r *vaultNetworkRepo) UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error {
	if id <= 0 {
		return fmt.Errorf("network not found")
	}
	res := r.db.WithContext(ctx).
		Model(&model.VaultNetworkModel{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(fields)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("network not found")
	}
	return nil
}
