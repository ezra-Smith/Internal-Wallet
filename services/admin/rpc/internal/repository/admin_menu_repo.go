package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"internalwallet/services/admin/rpc/internal/model"

	"gorm.io/gorm"
)

type AdminMenuRepository interface {
	WithTx(tx *gorm.DB) AdminMenuRepository
	GetDB() *gorm.DB

	Create(ctx context.Context, m *model.AdminMenuModel) error
	FindByID(ctx context.Context, id int64) (*model.AdminMenuModel, error)
	FindByIDs(ctx context.Context, ids []int64) ([]*model.AdminMenuModel, error)
	UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error
	SoftDelete(ctx context.Context, id int64, now time.Time) error
	CountActiveChildren(ctx context.Context, id int64) (int64, error)
	List(ctx context.Context, page, pageSize int32, status *int32, pid *int64, hideInMenu *bool) ([]*model.AdminMenuModel, int64, error)
	ListAllActiveForTree(ctx context.Context) ([]*model.AdminMenuModel, error)
	ListByPidIn(ctx context.Context, pids []int64) ([]*model.AdminMenuModel, error)
}

type adminMenuRepo struct {
	db *gorm.DB
}

func NewAdminMenuRepository(db *gorm.DB) AdminMenuRepository {
	return &adminMenuRepo{db: db}
}

func (r *adminMenuRepo) WithTx(tx *gorm.DB) AdminMenuRepository { return &adminMenuRepo{db: tx} }
func (r *adminMenuRepo) GetDB() *gorm.DB                        { return r.db }

func (r *adminMenuRepo) Create(ctx context.Context, m *model.AdminMenuModel) error {
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *adminMenuRepo) FindByID(ctx context.Context, id int64) (*model.AdminMenuModel, error) {
	var m model.AdminMenuModel
	err := r.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", id).
		First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("menu not found")
	}
	return &m, err
}

func (r *adminMenuRepo) FindByIDs(ctx context.Context, ids []int64) ([]*model.AdminMenuModel, error) {
	if len(ids) == 0 {
		return []*model.AdminMenuModel{}, nil
	}
	var items []*model.AdminMenuModel
	err := r.db.WithContext(ctx).
		Where("id IN ? AND deleted_at IS NULL", ids).
		Find(&items).Error
	return items, err
}

func (r *adminMenuRepo) UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error {
	res := r.db.WithContext(ctx).
		Model(&model.AdminMenuModel{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(fields)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("menu not found")
	}
	return nil
}

func (r *adminMenuRepo) SoftDelete(ctx context.Context, id int64, now time.Time) error {
	res := r.db.WithContext(ctx).
		Model(&model.AdminMenuModel{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]interface{}{
			"deleted_at": &now,
			"updated_at": &now,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("menu not found")
	}
	return nil
}

func (r *adminMenuRepo) CountActiveChildren(ctx context.Context, id int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.AdminMenuModel{}).
		Where("pid = ? AND deleted_at IS NULL", id).
		Count(&count).Error
	return count, err
}

func (r *adminMenuRepo) List(ctx context.Context, page, pageSize int32, status *int32, pid *int64, hideInMenu *bool) ([]*model.AdminMenuModel, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	q := r.db.WithContext(ctx).Model(&model.AdminMenuModel{}).Where("deleted_at IS NULL")
	if status != nil {
		q = q.Where("status = ?", *status)
	}
	if pid != nil {
		q = q.Where("pid = ?", *pid)
	}
	if hideInMenu != nil {
		v := 0
		if *hideInMenu {
			v = 1
		}
		q = q.Where("hide_in_menu = ?", v)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var list []*model.AdminMenuModel
	offset := int((page - 1) * pageSize)
	err := q.Order("sort ASC, created_at DESC").Offset(offset).Limit(int(pageSize)).Find(&list).Error
	return list, total, err
}

func (r *adminMenuRepo) ListAllActiveForTree(ctx context.Context) ([]*model.AdminMenuModel, error) {
	var list []*model.AdminMenuModel
	err := r.db.WithContext(ctx).
		Model(&model.AdminMenuModel{}).
		Where("deleted_at IS NULL AND status = 1").
		Order("sort ASC, id ASC").
		Find(&list).Error
	return list, err
}

func (r *adminMenuRepo) ListByPidIn(ctx context.Context, pids []int64) ([]*model.AdminMenuModel, error) {
	if len(pids) == 0 {
		return []*model.AdminMenuModel{}, nil
	}
	var list []*model.AdminMenuModel
	err := r.db.WithContext(ctx).
		Model(&model.AdminMenuModel{}).
		Where("pid IN ? AND deleted_at IS NULL", pids).
		Find(&list).Error
	return list, err
}
