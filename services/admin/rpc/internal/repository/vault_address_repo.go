package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"internalwallet/services/admin/rpc/internal/model"

	"gorm.io/gorm"
)

type VaultAddressRepository interface {
	WithTx(tx *gorm.DB) VaultAddressRepository
	GetDB() *gorm.DB

	Create(ctx context.Context, address *model.VaultAddressModel) error
	FindByID(ctx context.Context, id int64) (*model.VaultAddressModel, error)
	FindByAddress(ctx context.Context, networkID int64, address string) (*model.VaultAddressModel, error)
	List(ctx context.Context, filter *VaultAddressFilter, page, pageSize int32) ([]*model.VaultAddressModel, int64, error)
	UpdateStatus(ctx context.Context, id int64, status string, isActiveWallet int8) error
	Delete(ctx context.Context, id int64) error
	GetActiveWallet(ctx context.Context, networkID int64) (*model.VaultAddressModel, error)
}

type VaultAddressFilter struct {
	NetworkID   int64
	AddressType string
	Status      string
}

type vaultAddressRepo struct {
	db *gorm.DB
}

func NewVaultAddressRepository(db *gorm.DB) VaultAddressRepository {
	return &vaultAddressRepo{db: db}
}

func (r *vaultAddressRepo) WithTx(tx *gorm.DB) VaultAddressRepository {
	return &vaultAddressRepo{db: tx}
}

func (r *vaultAddressRepo) GetDB() *gorm.DB {
	return r.db
}

func (r *vaultAddressRepo) Create(ctx context.Context, address *model.VaultAddressModel) error {
	return r.db.WithContext(ctx).Create(address).Error
}

func (r *vaultAddressRepo) FindByID(ctx context.Context, id int64) (*model.VaultAddressModel, error) {
	if id == 0 {
		return nil, fmt.Errorf("address not found")
	}
	var m model.VaultAddressModel
	err := r.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", id).
		First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("address not found")
	}
	return &m, err
}

func (r *vaultAddressRepo) FindByAddress(ctx context.Context, networkID int64, address string) (*model.VaultAddressModel, error) {
	var m model.VaultAddressModel
	err := r.db.WithContext(ctx).
		Where("network_id = ? AND address = ? AND deleted_at IS NULL", networkID, address).
		First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil // 不存在返回 nil,nil
	}
	return &m, err
}

func (r *vaultAddressRepo) List(ctx context.Context, filter *VaultAddressFilter, page, pageSize int32) ([]*model.VaultAddressModel, int64, error) {
	q := r.db.WithContext(ctx).Model(&model.VaultAddressModel{}).Where("deleted_at IS NULL")

	if filter != nil {
		if filter.NetworkID > 0 {
			q = q.Where("network_id = ?", filter.NetworkID)
		}
		if strings.TrimSpace(filter.AddressType) != "" {
			q = q.Where("address_type = ?", strings.TrimSpace(filter.AddressType))
		}
		if strings.TrimSpace(filter.Status) != "" {
			q = q.Where("status = ?", strings.TrimSpace(filter.Status))
		}
	}

	// 获取总数
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	var items []*model.VaultAddressModel
	offset := (page - 1) * pageSize
	err := q.Order("created_at DESC").
		Limit(int(pageSize)).
		Offset(int(offset)).
		Find(&items).Error

	return items, total, err
}

func (r *vaultAddressRepo) UpdateStatus(ctx context.Context, id int64, status string, isActiveWallet int8) error {
	if id <= 0 {
		return fmt.Errorf("address not found")
	}

	updates := map[string]interface{}{
		"status": status,
	}

	// 如果设置为活跃钱包,需要先清除同一网络的其他活跃钱包
	if isActiveWallet == 1 {
		// 先获取当前地址的 network_id
		var addr model.VaultAddressModel
		if err := r.db.WithContext(ctx).Where("id = ?", id).First(&addr).Error; err != nil {
			return err
		}

		// 清除同一网络的其他活跃钱包
		if err := r.db.WithContext(ctx).
			Model(&model.VaultAddressModel{}).
			Where("network_id = ? AND id != ? AND deleted_at IS NULL", addr.NetworkID, id).
			Update("is_active_wallet", 0).Error; err != nil {
			return err
		}

		updates["is_active_wallet"] = 1
	}

	res := r.db.WithContext(ctx).
		Model(&model.VaultAddressModel{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(updates)

	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("address not found")
	}
	return nil
}

func (r *vaultAddressRepo) Delete(ctx context.Context, id int64) error {
	if id <= 0 {
		return fmt.Errorf("address not found")
	}
	res := r.db.WithContext(ctx).
		Model(&model.VaultAddressModel{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Update("deleted_at", gorm.Expr("NOW()"))
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("address not found")
	}
	return nil
}

func (r *vaultAddressRepo) GetActiveWallet(ctx context.Context, networkID int64) (*model.VaultAddressModel, error) {
	var m model.VaultAddressModel
	err := r.db.WithContext(ctx).
		Where("network_id = ? AND is_active_wallet = 1 AND deleted_at IS NULL", networkID).
		First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil // 没有活跃钱包返回 nil,nil
	}
	return &m, err
}
