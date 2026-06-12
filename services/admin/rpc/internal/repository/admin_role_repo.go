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

type AdminRoleRepository interface {
	WithTx(tx *gorm.DB) AdminRoleRepository
	GetDB() *gorm.DB

	Create(ctx context.Context, m *model.AdminRoleModel) error
	FindByID(ctx context.Context, id int64) (*model.AdminRoleModel, error)
	FindByIDs(ctx context.Context, ids []int64) ([]*model.AdminRoleModel, error)
	FindByCode(ctx context.Context, code string) (*model.AdminRoleModel, error)
	ExistsByCode(ctx context.Context, code string, excludeID int64) (bool, error)
	UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error
	SoftDelete(ctx context.Context, id int64, now time.Time) error
	List(ctx context.Context, page, pageSize int32, status *int32) ([]*model.AdminRoleModel, int64, error)
}

type adminRoleRepo struct {
	db *gorm.DB
}

func NewAdminRoleRepository(db *gorm.DB) AdminRoleRepository { return &adminRoleRepo{db: db} }
func (r *adminRoleRepo) WithTx(tx *gorm.DB) AdminRoleRepository {
	return &adminRoleRepo{db: tx}
}
func (r *adminRoleRepo) GetDB() *gorm.DB { return r.db }

func (r *adminRoleRepo) Create(ctx context.Context, m *model.AdminRoleModel) error {
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *adminRoleRepo) FindByID(ctx context.Context, id int64) (*model.AdminRoleModel, error) {
	var m model.AdminRoleModel
	err := r.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", id).
		First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("role not found")
	}
	return &m, err
}

func (r *adminRoleRepo) FindByIDs(ctx context.Context, ids []int64) ([]*model.AdminRoleModel, error) {
	if len(ids) == 0 {
		return []*model.AdminRoleModel{}, nil
	}
	var list []*model.AdminRoleModel
	err := r.db.WithContext(ctx).
		Where("id IN ? AND deleted_at IS NULL", ids).
		Find(&list).Error
	return list, err
}

func (r *adminRoleRepo) FindByCode(ctx context.Context, code string) (*model.AdminRoleModel, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, fmt.Errorf("role not found")
	}
	var m model.AdminRoleModel
	err := r.db.WithContext(ctx).
		Where("code = ? AND deleted_at IS NULL", code).
		First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("role not found")
	}
	return &m, err
}

func (r *adminRoleRepo) ExistsByCode(ctx context.Context, code string, excludeID int64) (bool, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return false, nil
	}
	q := r.db.WithContext(ctx).Model(&model.AdminRoleModel{}).Where("code = ? AND deleted_at IS NULL", code)
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *adminRoleRepo) UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error {
	res := r.db.WithContext(ctx).
		Model(&model.AdminRoleModel{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(fields)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("role not found")
	}
	return nil
}

func (r *adminRoleRepo) SoftDelete(ctx context.Context, id int64, now time.Time) error {
	res := r.db.WithContext(ctx).
		Model(&model.AdminRoleModel{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]interface{}{
			"deleted_at": &now,
			"updated_at": &now,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("role not found")
	}
	return nil
}

func (r *adminRoleRepo) List(ctx context.Context, page, pageSize int32, status *int32) ([]*model.AdminRoleModel, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	q := r.db.WithContext(ctx).Model(&model.AdminRoleModel{}).Where("deleted_at IS NULL")
	if status != nil {
		q = q.Where("status = ?", *status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []*model.AdminRoleModel
	offset := int((page - 1) * pageSize)
	err := q.Order("created_at DESC").Offset(offset).Limit(int(pageSize)).Find(&list).Error
	return list, total, err
}
