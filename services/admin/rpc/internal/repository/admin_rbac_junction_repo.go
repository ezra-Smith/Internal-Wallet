package repository

import (
	"context"

	"internalwallet/services/admin/rpc/internal/model"

	"gorm.io/gorm"
)

type AdminRoleMenuRepository interface {
	WithTx(tx *gorm.DB) AdminRoleMenuRepository
	GetDB() *gorm.DB
	CountByRoleID(ctx context.Context, roleID int64) (int64, error)
	DeleteByRoleID(ctx context.Context, roleID int64) error
}

type adminRoleMenuRepo struct {
	db *gorm.DB
}

func NewAdminRoleMenuRepository(db *gorm.DB) AdminRoleMenuRepository {
	return &adminRoleMenuRepo{db: db}
}
func (r *adminRoleMenuRepo) WithTx(tx *gorm.DB) AdminRoleMenuRepository {
	return &adminRoleMenuRepo{db: tx}
}
func (r *adminRoleMenuRepo) GetDB() *gorm.DB { return r.db }

func (r *adminRoleMenuRepo) CountByRoleID(ctx context.Context, roleID int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.AdminRoleMenuModel{}).Where("role_id = ?", roleID).Count(&count).Error
	return count, err
}

func (r *adminRoleMenuRepo) DeleteByRoleID(ctx context.Context, roleID int64) error {
	return r.db.WithContext(ctx).Where("role_id = ?", roleID).Delete(&model.AdminRoleMenuModel{}).Error
}

type AdminRolePermissionRepository interface {
	WithTx(tx *gorm.DB) AdminRolePermissionRepository
	GetDB() *gorm.DB
	CountByRoleID(ctx context.Context, roleID int64) (int64, error)
	CountByPermissionID(ctx context.Context, permissionID int64) (int64, error)
	DeleteByRoleID(ctx context.Context, roleID int64) error
	DeleteByPermissionID(ctx context.Context, permissionID int64) error
}

type adminRolePermissionRepo struct {
	db *gorm.DB
}

func NewAdminRolePermissionRepository(db *gorm.DB) AdminRolePermissionRepository {
	return &adminRolePermissionRepo{db: db}
}
func (r *adminRolePermissionRepo) WithTx(tx *gorm.DB) AdminRolePermissionRepository {
	return &adminRolePermissionRepo{db: tx}
}
func (r *adminRolePermissionRepo) GetDB() *gorm.DB { return r.db }

func (r *adminRolePermissionRepo) CountByRoleID(ctx context.Context, roleID int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.AdminRolePermissionModel{}).Where("role_id = ?", roleID).Count(&count).Error
	return count, err
}

func (r *adminRolePermissionRepo) CountByPermissionID(ctx context.Context, permissionID int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.AdminRolePermissionModel{}).Where("permission_id = ?", permissionID).Count(&count).Error
	return count, err
}

func (r *adminRolePermissionRepo) DeleteByRoleID(ctx context.Context, roleID int64) error {
	return r.db.WithContext(ctx).Where("role_id = ?", roleID).Delete(&model.AdminRolePermissionModel{}).Error
}

func (r *adminRolePermissionRepo) DeleteByPermissionID(ctx context.Context, permissionID int64) error {
	return r.db.WithContext(ctx).Where("permission_id = ?", permissionID).Delete(&model.AdminRolePermissionModel{}).Error
}

type AdminUserRoleRepository interface {
	WithTx(tx *gorm.DB) AdminUserRoleRepository
	GetDB() *gorm.DB
	CountByRoleID(ctx context.Context, roleID int64) (int64, error)
	CountByUserID(ctx context.Context, userID int64) (int64, error)
	DeleteByRoleID(ctx context.Context, roleID int64) error
	DeleteByUserID(ctx context.Context, userID int64) error
}

type adminUserRoleRepo struct {
	db *gorm.DB
}

func NewAdminUserRoleRepository(db *gorm.DB) AdminUserRoleRepository {
	return &adminUserRoleRepo{db: db}
}
func (r *adminUserRoleRepo) WithTx(tx *gorm.DB) AdminUserRoleRepository {
	return &adminUserRoleRepo{db: tx}
}
func (r *adminUserRoleRepo) GetDB() *gorm.DB { return r.db }

func (r *adminUserRoleRepo) CountByRoleID(ctx context.Context, roleID int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.AdminUserRoleModel{}).Where("role_id = ?", roleID).Count(&count).Error
	return count, err
}

func (r *adminUserRoleRepo) CountByUserID(ctx context.Context, userID int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.AdminUserRoleModel{}).Where("user_id = ?", userID).Count(&count).Error
	return count, err
}

func (r *adminUserRoleRepo) DeleteByRoleID(ctx context.Context, roleID int64) error {
	return r.db.WithContext(ctx).Where("role_id = ?", roleID).Delete(&model.AdminUserRoleModel{}).Error
}

func (r *adminUserRoleRepo) DeleteByUserID(ctx context.Context, userID int64) error {
	return r.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&model.AdminUserRoleModel{}).Error
}
