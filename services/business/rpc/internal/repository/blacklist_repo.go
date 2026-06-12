package repository

import (
	"context"
	"time"

	commonRepo "internalwallet/common/repository"
	"internalwallet/services/business/rpc/internal/model"

	"gorm.io/gorm"
)

// 黑名单标识类型常量
const (
	BlacklistIdentifierTypeEmail  int32 = 1 // 邮箱
	BlacklistIdentifierTypePhone  int32 = 2 // 手机号
	BlacklistIdentifierTypeGoogle int32 = 3 // Google ID
)

// 黑名单状态常量
const (
	BlacklistStatusActive   int32 = 1 // 生效中
	BlacklistStatusInactive int32 = 0 // 已解除
)

// BlacklistInfo 黑名单信息（用于返回给调用方）
type BlacklistInfo struct {
	IsBlacklisted bool      // 是否在黑名单中
	BlockedAt     time.Time // 被拉黑时间
	Reason        string    // 拉黑原因
	ExpiresAt     time.Time // 过期时间（如果设置了的话）
}

type BlacklistRepository interface {
	commonRepo.BaseRepository[model.BlacklistModel]

	// IsBlacklisted 检查标识是否在黑名单中
	// identifier: 邮箱/手机号/Google ID
	// identifierType: 1-邮箱, 2-手机号, 3-Google ID
	IsBlacklisted(ctx context.Context, identifier string, identifierType int32) (bool, error)

	// GetBlacklistInfo 获取黑名单详细信息
	GetBlacklistInfo(ctx context.Context, identifier string, identifierType int32) (*BlacklistInfo, error)
}

type blacklistRepo struct {
	commonRepo.BaseRepository[model.BlacklistModel]
}

func NewBlacklistRepository(db *gorm.DB) BlacklistRepository {
	return &blacklistRepo{
		BaseRepository: commonRepo.NewBaseRepository[model.BlacklistModel](db),
	}
}

// IsBlacklisted 检查标识是否在黑名单中
func (r *blacklistRepo) IsBlacklisted(ctx context.Context, identifier string, identifierType int32) (bool, error) {
	var count int64
	now := time.Now()

	err := r.GetDB().WithContext(ctx).
		Model(&model.BlacklistModel{}).
		Where("identifier = ? AND identifier_type = ? AND status = ?", identifier, identifierType, BlacklistStatusActive).
		Where("expires_at IS NULL OR expires_at > ?", now).
		Count(&count).Error

	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetBlacklistInfo 获取黑名单详细信息
func (r *blacklistRepo) GetBlacklistInfo(ctx context.Context, identifier string, identifierType int32) (*BlacklistInfo, error) {
	var record model.BlacklistModel
	now := time.Now()

	err := r.GetDB().WithContext(ctx).
		Where("identifier = ? AND identifier_type = ? AND status = ?", identifier, identifierType, BlacklistStatusActive).
		Where("expires_at IS NULL OR expires_at > ?", now).
		Order("created_at DESC").
		First(&record).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return &BlacklistInfo{IsBlacklisted: false}, nil
		}
		return nil, err
	}

	return &BlacklistInfo{
		IsBlacklisted: true,
		BlockedAt:     record.CreatedAt,
		Reason:        record.ReasonDetail,
		ExpiresAt:     record.ExpiresAt,
	}, nil
}
