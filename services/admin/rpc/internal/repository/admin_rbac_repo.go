package repository

import (
	"context"

	"internalwallet/services/admin/rpc/internal/model"

	"gorm.io/gorm"
)

// AdminRBACRepository provides join-based queries for RBAC v2.
type AdminRBACRepository interface {
	WithTx(tx *gorm.DB) AdminRBACRepository
	GetDB() *gorm.DB

	GetUserMenus(ctx context.Context, userID int64) ([]*model.AdminMenuModel, error)
	GetUserPermissions(ctx context.Context, userID int64) ([]*model.AdminPermissionModel, error)
	GetUserRoles(ctx context.Context, userID int64) ([]*model.AdminRoleModel, error)

	GetRoleMenus(ctx context.Context, roleID int64) ([]*model.AdminMenuModel, error)
	GetRolePermissions(ctx context.Context, roleID int64) ([]*model.AdminPermissionModel, error)
}

type adminRBACRepo struct {
	db *gorm.DB
}

func NewAdminRBACRepository(db *gorm.DB) AdminRBACRepository { return &adminRBACRepo{db: db} }
func (r *adminRBACRepo) WithTx(tx *gorm.DB) AdminRBACRepository {
	return &adminRBACRepo{db: tx}
}
func (r *adminRBACRepo) GetDB() *gorm.DB { return r.db }

func (r *adminRBACRepo) GetUserMenus(ctx context.Context, userID int64) ([]*model.AdminMenuModel, error) {
	var list []*model.AdminMenuModel
	err := r.db.WithContext(ctx).
		Table("admin_menu m").
		Select("DISTINCT m.*").
		Joins("JOIN admin_role_menu rm ON m.id = rm.menu_id").
		Joins("JOIN admin_user_role ur ON rm.role_id = ur.role_id").
		Where("ur.user_id = ? AND m.deleted_at IS NULL AND m.status = 1", userID).
		Order("m.sort ASC, m.created_at ASC").
		Find(&list).Error
	return list, err
}

func (r *adminRBACRepo) GetUserPermissions(ctx context.Context, userID int64) ([]*model.AdminPermissionModel, error) {
	var list []*model.AdminPermissionModel
	err := r.db.WithContext(ctx).
		Table("admin_permission p").
		Select("DISTINCT p.*").
		Joins("JOIN admin_role_permission rp ON p.id = rp.permission_id").
		Joins("JOIN admin_user_role ur ON rp.role_id = ur.role_id").
		Where("ur.user_id = ? AND p.deleted_at IS NULL AND p.status = 1", userID).
		Order("p.created_at DESC").
		Find(&list).Error
	return list, err
}

func (r *adminRBACRepo) GetUserRoles(ctx context.Context, userID int64) ([]*model.AdminRoleModel, error) {
	var list []*model.AdminRoleModel
	err := r.db.WithContext(ctx).
		Table("admin_role r").
		Select("DISTINCT r.*").
		Joins("JOIN admin_user_role ur ON r.id = ur.role_id").
		Where("ur.user_id = ? AND r.deleted_at IS NULL AND r.status = 1", userID).
		Order("r.id ASC").
		Find(&list).Error
	return list, err
}

func (r *adminRBACRepo) GetRoleMenus(ctx context.Context, roleID int64) ([]*model.AdminMenuModel, error) {
	var list []*model.AdminMenuModel
	err := r.db.WithContext(ctx).
		Table("admin_menu m").
		Select("m.*").
		Joins("JOIN admin_role_menu rm ON m.id = rm.menu_id").
		Where("rm.role_id = ? AND m.deleted_at IS NULL AND m.status = 1", roleID).
		Order("m.sort ASC, m.created_at ASC").
		Find(&list).Error
	return list, err
}

func (r *adminRBACRepo) GetRolePermissions(ctx context.Context, roleID int64) ([]*model.AdminPermissionModel, error) {
	var list []*model.AdminPermissionModel
	err := r.db.WithContext(ctx).
		Table("admin_permission p").
		Select("p.*").
		Joins("JOIN admin_role_permission rp ON p.id = rp.permission_id").
		Where("rp.role_id = ? AND p.deleted_at IS NULL AND p.status = 1", roleID).
		Order("p.created_at DESC").
		Find(&list).Error
	return list, err
}
