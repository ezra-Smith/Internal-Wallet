package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	commonRepo "internalwallet/common/repository"
	"internalwallet/services/admin/rpc/internal/model"

	"gorm.io/gorm"
)

type UserRepository interface {
	commonRepo.BaseRepository[model.UserModel]

	GetByUID(ctx context.Context, uid string) (*model.UserModel, error)
	List(ctx context.Context, page, pageSize int32, status, role, keyword string, createdFrom, createdTo *time.Time, sortBy, sortOrder string, twoFactorEnabled, bypassWithdrawAudit *bool) ([]*model.UserModel, int64, error)
	UpdateStatus(ctx context.Context, id int64, status int32) error
	UpdateMemberLevel(ctx context.Context, id int64, memberLevel int32) error
}

type userRepo struct {
	commonRepo.BaseRepository[model.UserModel]
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepo{
		BaseRepository: commonRepo.NewBaseRepository[model.UserModel](db),
	}
}

func (r *userRepo) FindByID(ctx context.Context, id int64) (*model.UserModel, error) {
	if id <= 0 {
		return nil, fmt.Errorf("invalid id")
	}
	var entity model.UserModel
	err := r.GetDB().WithContext(ctx).
		Model(&model.UserModel{}).
		Where("id = ? AND deleted_at IS NULL", id).
		First(&entity).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("record not found")
	}
	return &entity, err
}

func (r *userRepo) GetByUID(ctx context.Context, uid string) (*model.UserModel, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(uid), 10, 64)
	if err != nil || id <= 0 {
		return nil, fmt.Errorf("invalid uid")
	}
	return r.FindByID(ctx, id)
}

func (r *userRepo) List(ctx context.Context, page, pageSize int32, status, role, keyword string, createdFrom, createdTo *time.Time, sortBy, sortOrder string, twoFactorEnabled, bypassWithdrawAudit *bool) ([]*model.UserModel, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	query := r.GetDB().WithContext(ctx).Model(&model.UserModel{}).Where("users.deleted_at IS NULL")

	// Filter: 2FA enabled (best-effort via member_security_setting.google_auth_enabled; fallback to users.is_2fa_enabled)
	if twoFactorEnabled != nil {
		query = query.
			Joins("LEFT JOIN member_security_setting mss ON mss.user_id = users.id AND mss.deleted_at IS NULL").
			Where("IFNULL(mss.google_auth_enabled, users.is_2fa_enabled) = ?", *twoFactorEnabled)
	}

	// Filter: withdraw-audit bypass whitelist (union of legacy global flag + per-rule enabled entries).
	if bypassWithdrawAudit != nil {
		expr := `(EXISTS (SELECT 1 FROM user_withdraw_audit_whitelist_rules uwr WHERE uwr.user_id = users.id AND uwr.deleted_at IS NULL AND uwr.enabled = 1)
OR EXISTS (SELECT 1 FROM user_whitelist_settings uws WHERE uws.user_id = users.id AND uws.deleted_at IS NULL AND uws.bypass_withdraw_audit = 1))`
		if *bypassWithdrawAudit {
			query = query.Where(expr)
		} else {
			query = query.Where("NOT " + expr)
		}
	}

	// status filter (1-active 2-frozen 3-terminated)
	switch strings.TrimSpace(status) {
	case "":
		// no-op
	case "active":
		// 查询所有正常状态的用户（包括待KYC和已KYC）
		// 前端已隐藏"待KYC"选项，"正常"状态包含所有 status=1 的用户
		query = query.Where("status = ?", 1)
	case "pending_kyc":
		// 保留此选项以保持向后兼容，虽然前端不再使用
		query = query.Where("status = ?", 1).Where("kyc_level = 0")
	case "frozen":
		query = query.Where("status = ?", 2)
	case "terminated":
		query = query.Where("status = ?", 3)
	default:
		return nil, 0, fmt.Errorf("invalid status")
	}

	// role filter (mapped from member_level)
	switch strings.TrimSpace(role) {
	case "":
	case "user":
		query = query.Where("member_level < ?", 2)
	case "vip":
		query = query.Where("member_level >= ?", 2)
	default:
		return nil, 0, fmt.Errorf("invalid role")
	}

	kw := strings.TrimSpace(keyword)
	if kw != "" {
		if id, err := strconv.ParseInt(kw, 10, 64); err == nil && id > 0 {
			query = query.Where("(id = ? OR email LIKE ? OR phone LIKE ?)", id, "%"+kw+"%", "%"+kw+"%")
		} else {
			like := "%" + kw + "%"
			query = query.Where("(email LIKE ? OR phone LIKE ?)", like, like)
		}
	}

	if createdFrom != nil && !createdFrom.IsZero() {
		query = query.Where("created_at >= ?", *createdFrom)
	}
	if createdTo != nil && !createdTo.IsZero() {
		query = query.Where("created_at <= ?", *createdTo)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	orderCol := "created_at"
	switch strings.TrimSpace(sortBy) {
	case "", "created_at":
		orderCol = "created_at"
	case "last_login":
		orderCol = "last_login_time"
	case "balance":
		// not supported yet, fallback
		orderCol = "created_at"
	default:
		return nil, 0, fmt.Errorf("invalid sort_by")
	}

	orderDir := "DESC"
	if strings.EqualFold(strings.TrimSpace(sortOrder), "asc") {
		orderDir = "ASC"
	} else if strings.EqualFold(strings.TrimSpace(sortOrder), "desc") || strings.TrimSpace(sortOrder) == "" {
		orderDir = "DESC"
	} else {
		return nil, 0, fmt.Errorf("invalid sort_order")
	}

	var items []*model.UserModel
	offset := int((page - 1) * pageSize)
	if err := query.Order(orderCol + " " + orderDir).Offset(offset).Limit(int(pageSize)).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *userRepo) UpdateStatus(ctx context.Context, id int64, status int32) error {
	res := r.GetDB().WithContext(ctx).
		Model(&model.UserModel{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]interface{}{
			"status":     status,
			"updated_at": time.Now(),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("user not found")
	}
	return nil
}

func (r *userRepo) UpdateMemberLevel(ctx context.Context, id int64, memberLevel int32) error {
	res := r.GetDB().WithContext(ctx).
		Model(&model.UserModel{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]interface{}{
			"member_level": memberLevel,
			"updated_at":   time.Now(),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("user not found")
	}
	return nil
}
