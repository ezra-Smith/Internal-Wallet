package repository

import (
	"context"
	"errors"
	"strconv"

	commonRepo "internalwallet/common/repository"
	"internalwallet/services/business/rpc/internal/model"

	"gorm.io/gorm"
)

// MemberInternalAddressRepository 用户内部地址簿 Repository 接口
type MemberInternalAddressRepository interface {
	commonRepo.BaseRepository[model.MemberInternalAddressModel]

	// 列出用户的内部地址
	ListByUserID(ctx context.Context, userID int64) ([]model.MemberInternalAddressModel, error)
	// 查找用户对某目标用户的内部地址
	FindByUserAndTarget(ctx context.Context, userID int64, targetUserID int64) (*model.MemberInternalAddressModel, error)
	// 创建内部地址
	Create(ctx context.Context, addr *model.MemberInternalAddressModel) error
	// 取消用户所有内部地址的默认状态
	SetAllDefaultFalseByUser(ctx context.Context, userID int64) error
}

type memberInternalAddressRepo struct {
	commonRepo.BaseRepository[model.MemberInternalAddressModel]
}

// NewMemberInternalAddressRepository 创建 Repository
func NewMemberInternalAddressRepository(db *gorm.DB) MemberInternalAddressRepository {
	return &memberInternalAddressRepo{
		BaseRepository: commonRepo.NewBaseRepository[model.MemberInternalAddressModel](db),
	}
}

// ListByUserID 列出用户的内部地址
func (r *memberInternalAddressRepo) ListByUserID(ctx context.Context, userID int64) ([]model.MemberInternalAddressModel, error) {
	var rows []model.MemberInternalAddressModel
	err := r.GetDB().WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&rows).Error
	return rows, err
}

// FindByUserAndTarget 查找用户对某目标用户的内部地址
func (r *memberInternalAddressRepo) FindByUserAndTarget(ctx context.Context, userID int64, targetUserID int64) (*model.MemberInternalAddressModel, error) {
	var m model.MemberInternalAddressModel
	err := r.GetDB().WithContext(ctx).
		Where("user_id = ? AND target_user_id = ?", userID, targetUserID).
		First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// Create 创建内部地址
func (r *memberInternalAddressRepo) Create(ctx context.Context, addr *model.MemberInternalAddressModel) error {
	return r.GetDB().WithContext(ctx).Create(addr).Error
}

// SetAllDefaultFalseByUser 取消用户所有内部地址的默认状态
func (r *memberInternalAddressRepo) SetAllDefaultFalseByUser(ctx context.Context, userID int64) error {
	return r.GetDB().WithContext(ctx).
		Model(&model.MemberInternalAddressModel{}).
		Where("user_id = ?", userID).
		Updates(map[string]interface{}{"is_default": false}).Error
}

// MaskEmail 邮箱掩码处理
func MaskEmail(email string) string {
	if email == "" {
		return ""
	}
	at := -1
	for i, c := range email {
		if c == '@' {
			at = i
			break
		}
	}
	if at <= 0 {
		return email
	}
	// 显示前2个字符和@后面的部分
	prefix := email[:at]
	suffix := email[at:]
	if len(prefix) <= 2 {
		return prefix + "***" + suffix
	}
	return prefix[:2] + "***" + suffix
}

// MaskUID UID掩码处理
// 规则：1-2 位数的 UID 不脱敏（如 "1", "14"），3 位数及以上的 UID 进行脱敏（如 "101" -> "10****1", "1014" -> "10****14"）
func MaskUID(uid string) string {
	if uid == "" {
		return ""
	}
	// 1-2 位数的 UID 不脱敏，直接显示
	if len(uid) <= 2 {
		return uid
	}
	// 3 位数及以上的 UID 进行脱敏
	return uid[:2] + "****" + uid[len(uid)-2:]
}

// FormatTargetUserDisplay 格式化目标用户显示
func FormatTargetUserDisplay(targetUserID int64, email string) string {
	if email != "" {
		return MaskEmail(email)
	}
	return MaskUID(strconv.FormatInt(targetUserID, 10))
}
