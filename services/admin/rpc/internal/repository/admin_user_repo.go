package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	commonRepo "internalwallet/common/repository"
	"internalwallet/services/admin/rpc/internal/model"

	"gorm.io/gorm"
)

type AdminUserRepository interface {
	commonRepo.BaseRepository[model.AdminUserModel]

	GetByUsername(ctx context.Context, username string) (*model.AdminUserModel, error)
	List(ctx context.Context, page, pageSize int32, role, status, keyword string) ([]*model.AdminUserModel, int64, error)
	CountActiveSuperAdmins(ctx context.Context) (int64, error)

	IncrementFailedLogin(ctx context.Context, id int64, lockUntil *time.Time) (int, error)
	ResetFailedLogin(ctx context.Context, id int64) error
	UpdateLoginSuccess(ctx context.Context, id int64, ip string) error
	SetLockUntil(ctx context.Context, id int64, lockUntil *time.Time, updatedBy int64) error

	UpdatePassword(ctx context.Context, id int64, passwordHash string, requireChange bool, updatedBy int64) error
	UpdateRole(ctx context.Context, id int64, role string, updatedBy int64) error
	UpdateStatus(ctx context.Context, id int64, status string, updatedBy int64) error
	UpdateTwoFactorSecret(ctx context.Context, id int64, secret string, updatedBy int64) error
	UpdateTwoFactorPendingSecret(ctx context.Context, id int64, secret string, updatedBy int64) error
	SetTwoFactorEnabled(ctx context.Context, id int64, enabled bool, updatedBy int64) error
	ResetTwoFactor(ctx context.Context, id int64, updatedBy int64) error
	PromoteTwoFactorPending(ctx context.Context, id int64, activeSecret string, boundAt *time.Time, updatedBy int64) error
	DisableTwoFactor(ctx context.Context, id int64, updatedBy int64) error

	IncrementTokenVersion(ctx context.Context, id int64, updatedBy int64) (int64, error)
}

type adminUserRepo struct {
	commonRepo.BaseRepository[model.AdminUserModel]
}

func NewAdminUserRepository(db *gorm.DB) AdminUserRepository {
	return &adminUserRepo{
		BaseRepository: commonRepo.NewBaseRepository[model.AdminUserModel](db),
	}
}

func (r *adminUserRepo) GetByUsername(ctx context.Context, username string) (*model.AdminUserModel, error) {
	var m model.AdminUserModel
	err := r.GetDB().WithContext(ctx).
		Where("username = ?", username).
		First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("admin user not found")
	}
	return &m, err
}

func (r *adminUserRepo) List(ctx context.Context, page, pageSize int32, role, status, keyword string) ([]*model.AdminUserModel, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	query := r.GetDB().WithContext(ctx).Model(&model.AdminUserModel{})
	if role != "" {
		query = query.Where("role = ?", role)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if keyword != "" {
		kw := "%" + strings.TrimSpace(keyword) + "%"
		query = query.Where("(username LIKE ? OR name LIKE ?)", kw, kw)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []*model.AdminUserModel
	offset := int((page - 1) * pageSize)
	err := query.Order("id DESC").Offset(offset).Limit(int(pageSize)).Find(&items).Error
	return items, total, err
}

func (r *adminUserRepo) CountActiveSuperAdmins(ctx context.Context) (int64, error) {
	var count int64
	err := r.GetDB().WithContext(ctx).
		Table("admin_users u").
		Joins("JOIN admin_user_role ur ON u.id = ur.user_id").
		Joins("JOIN admin_role r ON ur.role_id = r.id").
		Where("u.deleted_at IS NULL AND ur.deleted_at IS NULL AND u.status = ? AND r.code = ? AND r.deleted_at IS NULL AND r.status = 1", "active", "super_admin").
		Distinct("u.id").
		Count(&count).Error
	return count, err
}

func (r *adminUserRepo) IncrementFailedLogin(ctx context.Context, id int64, lockUntil *time.Time) (int, error) {
	updates := map[string]interface{}{
		"failed_login_count": gorm.Expr("failed_login_count + 1"),
	}
	if lockUntil != nil {
		updates["lock_until"] = lockUntil
	}

	if err := r.GetDB().WithContext(ctx).
		Model(&model.AdminUserModel{}).
		Where("id = ?", id).
		Updates(updates).Error; err != nil {
		return 0, err
	}

	var m model.AdminUserModel
	if err := r.GetDB().WithContext(ctx).
		Select("failed_login_count").
		Where("id = ?", id).
		First(&m).Error; err != nil {
		return 0, err
	}
	return m.FailedLoginCount, nil
}

func (r *adminUserRepo) ResetFailedLogin(ctx context.Context, id int64) error {
	return r.GetDB().WithContext(ctx).
		Model(&model.AdminUserModel{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"failed_login_count": 0,
			"lock_until":         nil,
		}).Error
}

func (r *adminUserRepo) UpdateLoginSuccess(ctx context.Context, id int64, ip string) error {
	now := time.Now()
	return r.GetDB().WithContext(ctx).
		Model(&model.AdminUserModel{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"last_login_at":      &now,
			"last_login_ip":      ip,
			"failed_login_count": 0,
			"lock_until":         nil,
		}).Error
}

func (r *adminUserRepo) SetLockUntil(ctx context.Context, id int64, lockUntil *time.Time, updatedBy int64) error {
	return r.GetDB().WithContext(ctx).
		Model(&model.AdminUserModel{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"lock_until": lockUntil,
			"updated_by": updatedBy,
			"updated_at": time.Now().Local(),
		}).Error
}

func (r *adminUserRepo) UpdatePassword(ctx context.Context, id int64, passwordHash string, requireChange bool, updatedBy int64) error {
	now := time.Now()
	return r.GetDB().WithContext(ctx).
		Model(&model.AdminUserModel{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"password_hash":           passwordHash,
			"require_password_change": requireChange,
			"password_changed_at":     &now,
			"updated_by":              updatedBy,
		}).Error
}

func (r *adminUserRepo) UpdateRole(ctx context.Context, id int64, role string, updatedBy int64) error {
	return r.GetDB().WithContext(ctx).
		Model(&model.AdminUserModel{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"role":       role,
			"updated_by": updatedBy,
		}).Error
}

func (r *adminUserRepo) UpdateStatus(ctx context.Context, id int64, status string, updatedBy int64) error {
	return r.GetDB().WithContext(ctx).
		Model(&model.AdminUserModel{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":     status,
			"updated_by": updatedBy,
		}).Error
}

func (r *adminUserRepo) UpdateTwoFactorSecret(ctx context.Context, id int64, secret string, updatedBy int64) error {
	return r.GetDB().WithContext(ctx).
		Model(&model.AdminUserModel{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"two_factor_secret": secret,
			"updated_by":        updatedBy,
		}).Error
}

func (r *adminUserRepo) UpdateTwoFactorPendingSecret(ctx context.Context, id int64, secret string, updatedBy int64) error {
	return r.GetDB().WithContext(ctx).
		Model(&model.AdminUserModel{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"two_factor_pending_secret": secret,
			"updated_by":                updatedBy,
		}).Error
}

func (r *adminUserRepo) SetTwoFactorEnabled(ctx context.Context, id int64, enabled bool, updatedBy int64) error {
	v := 0
	if enabled {
		v = 1
	}
	return r.GetDB().WithContext(ctx).
		Model(&model.AdminUserModel{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"two_factor_enabled": v,
			"updated_by":         updatedBy,
		}).Error
}

func (r *adminUserRepo) ResetTwoFactor(ctx context.Context, id int64, updatedBy int64) error {
	return r.GetDB().WithContext(ctx).
		Model(&model.AdminUserModel{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"two_factor_secret":         "",
			"two_factor_pending_secret": "",
			"two_factor_enabled":        0,
			"two_factor_bound_at":       nil,
			"updated_by":                updatedBy,
		}).Error
}

func (r *adminUserRepo) PromoteTwoFactorPending(ctx context.Context, id int64, activeSecret string, boundAt *time.Time, updatedBy int64) error {
	return r.GetDB().WithContext(ctx).
		Model(&model.AdminUserModel{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"two_factor_secret":         activeSecret,
			"two_factor_pending_secret": "",
			"two_factor_enabled":        1,
			"two_factor_bound_at":       boundAt,
			"updated_by":                updatedBy,
		}).Error
}

func (r *adminUserRepo) DisableTwoFactor(ctx context.Context, id int64, updatedBy int64) error {
	return r.GetDB().WithContext(ctx).
		Model(&model.AdminUserModel{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"two_factor_secret":         "",
			"two_factor_pending_secret": "",
			"two_factor_enabled":        0,
			"two_factor_bound_at":       nil,
			"updated_by":                updatedBy,
		}).Error
}

func (r *adminUserRepo) IncrementTokenVersion(ctx context.Context, id int64, updatedBy int64) (int64, error) {
	if err := r.GetDB().WithContext(ctx).
		Model(&model.AdminUserModel{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"token_version": gorm.Expr("token_version + 1"),
			"updated_by":    updatedBy,
		}).Error; err != nil {
		return 0, err
	}

	var m model.AdminUserModel
	if err := r.GetDB().WithContext(ctx).
		Select("token_version").
		Where("id = ?", id).
		First(&m).Error; err != nil {
		return 0, err
	}
	return m.TokenVersion, nil
}
