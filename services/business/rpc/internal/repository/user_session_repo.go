package repository

import (
	"context"
	"time"

	"gorm.io/gorm"
	commonRepo "internalwallet/common/repository"
	"internalwallet/services/business/rpc/internal/model"
)

type UserSessionRepository interface {
	commonRepo.BaseRepository[model.UserSessionModel]

	// 根据 RefreshToken 查找会话
	FindByRefreshToken(ctx context.Context, refreshToken string) (*model.UserSessionModel, error)

	// 根据 SessionToken 查找会话
	FindBySessionToken(ctx context.Context, sessionToken string) (*model.UserSessionModel, error)

	// 根据用户ID和设备ID查找会话
	FindByUserAndDevice(ctx context.Context, userID int64, deviceID string) (*model.UserSessionModel, error)

	// 更新会话令牌（令牌轮换）
	UpdateTokens(ctx context.Context, sessionID int64, newSessionToken, newRefreshToken string, expiresAt time.Time) error

	// 使用户的所有会话失效
	InvalidateUserSessions(ctx context.Context, userID int64) error

	// 使特定会话失效
	InvalidateSession(ctx context.Context, sessionID int64) error

	// 清理过期会话
	CleanupExpiredSessions(ctx context.Context) error

	// 获取用户的所有活跃会话
	GetActiveSessionsByUser(ctx context.Context, userID int64) ([]model.UserSessionModel, error)
}

type userSessionRepo struct {
	commonRepo.BaseRepository[model.UserSessionModel]
	db *gorm.DB
}

func NewUserSessionRepository(db *gorm.DB) UserSessionRepository {
	return &userSessionRepo{
		BaseRepository: commonRepo.NewBaseRepository[model.UserSessionModel](db),
		db:             db,
	}
}

// FindByRefreshToken 根据 RefreshToken 查找会话
func (r *userSessionRepo) FindByRefreshToken(ctx context.Context, refreshToken string) (*model.UserSessionModel, error) {
	var session model.UserSessionModel
	err := r.db.WithContext(ctx).
		Where("refresh_token = ? AND is_active = ? AND expires_at > ?", refreshToken, true, time.Now()).
		First(&session).Error

	if err != nil {
		return nil, err
	}
	return &session, nil
}

// FindBySessionToken 根据 SessionToken 查找会话
func (r *userSessionRepo) FindBySessionToken(ctx context.Context, sessionToken string) (*model.UserSessionModel, error) {
	var session model.UserSessionModel
	err := r.db.WithContext(ctx).
		Where("session_token = ? AND is_active = ? AND expires_at > ?", sessionToken, true, time.Now()).
		First(&session).Error

	if err != nil {
		return nil, err
	}
	return &session, nil
}

// FindByUserAndDevice 根据用户ID和设备ID查找会话
func (r *userSessionRepo) FindByUserAndDevice(ctx context.Context, userID int64, deviceID string) (*model.UserSessionModel, error) {
	var session model.UserSessionModel
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND device_id = ? AND is_active = ?", userID, deviceID, true).
		Order("created_at DESC").
		First(&session).Error

	if err != nil {
		return nil, err
	}
	return &session, nil
}

// UpdateTokens 更新会话令牌（令牌轮换）
func (r *userSessionRepo) UpdateTokens(ctx context.Context, sessionID int64, newSessionToken, newRefreshToken string, expiresAt time.Time) error {
	return r.db.WithContext(ctx).
		Model(&model.UserSessionModel{}).
		Where("id = ?", sessionID).
		Updates(map[string]interface{}{
			"session_token": newSessionToken,
			"refresh_token": newRefreshToken,
			"expires_at":    expiresAt,
			"updated_at":    time.Now(),
		}).Error
}

// InvalidateUserSessions 使用户的所有会话失效
func (r *userSessionRepo) InvalidateUserSessions(ctx context.Context, userID int64) error {
	return r.db.WithContext(ctx).
		Model(&model.UserSessionModel{}).
		Where("user_id = ? AND is_active = ?", userID, true).
		Updates(map[string]interface{}{
			"is_active":  false,
			"updated_at": time.Now(),
		}).Error
}

// InvalidateSession 使特定会话失效
func (r *userSessionRepo) InvalidateSession(ctx context.Context, sessionID int64) error {
	return r.db.WithContext(ctx).
		Model(&model.UserSessionModel{}).
		Where("id = ?", sessionID).
		Updates(map[string]interface{}{
			"is_active":  false,
			"updated_at": time.Now(),
		}).Error
}

// CleanupExpiredSessions 清理过期会话
func (r *userSessionRepo) CleanupExpiredSessions(ctx context.Context) error {
	// 删除30天前过期的会话记录
	threshold := time.Now().AddDate(0, 0, -30)
	return r.db.WithContext(ctx).
		Where("expires_at < ? AND is_active = ?", threshold, false).
		Delete(&model.UserSessionModel{}).Error
}

// GetActiveSessionsByUser 获取用户的所有活跃会话
func (r *userSessionRepo) GetActiveSessionsByUser(ctx context.Context, userID int64) ([]model.UserSessionModel, error) {
	var sessions []model.UserSessionModel
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND is_active = ? AND expires_at > ?", userID, true, time.Now()).
		Order("created_at DESC").
		Find(&sessions).Error

	return sessions, err
}
