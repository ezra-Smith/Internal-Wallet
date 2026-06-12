package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"internalwallet/services/admin/rpc/internal/model"

	"gorm.io/gorm"
)

type AdminPermissionRepository interface {
	WithTx(tx *gorm.DB) AdminPermissionRepository
	GetDB() *gorm.DB

	Create(ctx context.Context, m *model.AdminPermissionModel) error
	FindByID(ctx context.Context, id int64) (*model.AdminPermissionModel, error)
	FindByCode(ctx context.Context, code string) (*model.AdminPermissionModel, error)
	ExistsByCode(ctx context.Context, code string, excludeID int64) (bool, error)
	UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error
	SoftDelete(ctx context.Context, id int64, now time.Time) error
	List(ctx context.Context, page, pageSize int32, menuID *int64, status *int32, name, code, description string) ([]*model.AdminPermissionModel, int64, error)
	ListActiveByMenuID(ctx context.Context, menuID int64) ([]*model.AdminPermissionModel, error)
	FindByIDs(ctx context.Context, ids []int64) ([]*model.AdminPermissionModel, error)
}

type adminPermissionRepo struct {
	db *gorm.DB
}

func NewAdminPermissionRepository(db *gorm.DB) AdminPermissionRepository {
	return &adminPermissionRepo{db: db}
}

func (r *adminPermissionRepo) WithTx(tx *gorm.DB) AdminPermissionRepository {
	return &adminPermissionRepo{db: tx}
}
func (r *adminPermissionRepo) GetDB() *gorm.DB { return r.db }

func (r *adminPermissionRepo) Create(ctx context.Context, m *model.AdminPermissionModel) error {
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *adminPermissionRepo) FindByID(ctx context.Context, id int64) (*model.AdminPermissionModel, error) {
	var m model.AdminPermissionModel
	err := r.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("permission not found")
	}
	return &m, err
}

func (r *adminPermissionRepo) FindByIDs(ctx context.Context, ids []int64) ([]*model.AdminPermissionModel, error) {
	if len(ids) == 0 {
		return []*model.AdminPermissionModel{}, nil
	}
	var list []*model.AdminPermissionModel
	err := r.db.WithContext(ctx).
		Where("id IN ? AND deleted_at IS NULL", ids).
		Find(&list).Error
	return list, err
}

func (r *adminPermissionRepo) FindByCode(ctx context.Context, code string) (*model.AdminPermissionModel, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, fmt.Errorf("permission not found")
	}
	var m model.AdminPermissionModel
	err := r.db.WithContext(ctx).
		Where("code = ? AND deleted_at IS NULL", code).
		First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("permission not found")
	}
	return &m, err
}

func (r *adminPermissionRepo) ExistsByCode(ctx context.Context, code string, excludeID int64) (bool, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return false, nil
	}
	q := r.db.WithContext(ctx).Model(&model.AdminPermissionModel{}).Where("code = ? AND deleted_at IS NULL", code)
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *adminPermissionRepo) UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error {
	res := r.db.WithContext(ctx).
		Model(&model.AdminPermissionModel{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(fields)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("permission not found")
	}
	return nil
}

func (r *adminPermissionRepo) SoftDelete(ctx context.Context, id int64, now time.Time) error {
	res := r.db.WithContext(ctx).
		Model(&model.AdminPermissionModel{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]interface{}{
			"deleted_at": &now,
			"updated_at": &now,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("permission not found")
	}
	return nil
}

func (r *adminPermissionRepo) List(ctx context.Context, page, pageSize int32, menuID *int64, status *int32, name, code, description string) ([]*model.AdminPermissionModel, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	q := r.db.WithContext(ctx).Model(&model.AdminPermissionModel{}).Where("deleted_at IS NULL")
	if menuID != nil {
		q = q.Where("menu_id = ?", *menuID)
	}
	if status != nil {
		q = q.Where("status = ?", *status)
	}
	if name != "" {
		q = q.Where("name LIKE ?", "%"+name+"%")
	}
	if code != "" {
		q = q.Where("code LIKE ?", "%"+code+"%")
	}
	if description != "" {
		q = q.Where("description LIKE ?", "%"+description+"%")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []*model.AdminPermissionModel
	offset := int((page - 1) * pageSize)
	err := q.Order("created_at DESC").Offset(offset).Limit(int(pageSize)).Find(&list).Error
	return list, total, err
}

func (r *adminPermissionRepo) ListActiveByMenuID(ctx context.Context, menuID int64) ([]*model.AdminPermissionModel, error) {
	var list []*model.AdminPermissionModel
	err := r.db.WithContext(ctx).
		Model(&model.AdminPermissionModel{}).
		Where("menu_id = ? AND deleted_at IS NULL AND status = 1", menuID).
		Order("created_at DESC").
		Find(&list).Error
	return list, err
}
