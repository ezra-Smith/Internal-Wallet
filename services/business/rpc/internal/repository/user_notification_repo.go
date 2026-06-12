package repository

import (
	"context"
	"encoding/json"
	"time"

	commonRepo "internalwallet/common/repository"
	"internalwallet/services/business/rpc/internal/model"

	"gorm.io/gorm"
)

type UserNotificationRepository interface {
	commonRepo.BaseRepository[model.UserNotificationModel]

	// 用户消息通知
	CountUnreadByUser(ctx context.Context, userID int64) (int64, error)
	ListNotificationsByUser(ctx context.Context, userID int64, page int32, size int32) ([]model.UserNotificationModel, int64, error)
	ListNotifications(ctx context.Context, userID int64, onlyUnread bool, page int32, size int32) ([]model.UserNotificationModel, int64, error)
	MarkRead(ctx context.Context, id int64, userID int64) error
	MarkAllReadByUser(ctx context.Context, userID int64) error
	DeleteByUser(ctx context.Context, id int64, userID int64) error
	ClearAllByUser(ctx context.Context, userID int64) (int64, error)

	// 创建消息（内部使用）
	CreateNotification(ctx context.Context, userID int64, msgType, title, content string, data map[string]interface{}) error
	BatchCreateNotifications(ctx context.Context, userIDs []int64, msgType, title, content string, data map[string]interface{}) error
}

type userNotificationRepo struct {
	commonRepo.BaseRepository[model.UserNotificationModel]
}

func NewUserNotificationRepository(db *gorm.DB) UserNotificationRepository {
	return &userNotificationRepo{
		BaseRepository: commonRepo.NewBaseRepository[model.UserNotificationModel](db),
	}
}

func (r *userNotificationRepo) CountUnreadByUser(ctx context.Context, userID int64) (int64, error) {
	var count int64
	err := r.GetDB().WithContext(ctx).Model(&model.UserNotificationModel{}).
		Where("user_id = ? AND `read` = 0", userID).
		Count(&count).Error
	return count, err
}

func (r *userNotificationRepo) ListNotificationsByUser(ctx context.Context, userID int64, page int32, size int32) ([]model.UserNotificationModel, int64, error) {
	q := r.GetDB().WithContext(ctx).Model(&model.UserNotificationModel{}).
		Where("user_id = ?", userID)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []model.UserNotificationModel
	err := q.Order("created_at DESC").Offset(int((page - 1) * size)).Limit(int(size)).Find(&rows).Error
	return rows, total, err
}

func (r *userNotificationRepo) ListNotifications(ctx context.Context, userID int64, onlyUnread bool, page int32, size int32) ([]model.UserNotificationModel, int64, error) {
	q := r.GetDB().WithContext(ctx).Model(&model.UserNotificationModel{}).
		Where("user_id = ?", userID)
	if onlyUnread {
		q = q.Where("`read` = 0")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []model.UserNotificationModel
	err := q.Order("created_at DESC").Offset(int((page - 1) * size)).Limit(int(size)).Find(&rows).Error
	return rows, total, err
}

func (r *userNotificationRepo) MarkRead(ctx context.Context, id int64, userID int64) error {
	now := time.Now()
	return r.GetDB().WithContext(ctx).Model(&model.UserNotificationModel{}).
		Where("id = ? AND user_id = ?", id, userID).
		Updates(map[string]interface{}{"read": true, "read_at": now}).Error
}

func (r *userNotificationRepo) MarkAllReadByUser(ctx context.Context, userID int64) error {
	now := time.Now()
	return r.GetDB().WithContext(ctx).Model(&model.UserNotificationModel{}).
		Where("user_id = ? AND `read` = 0", userID).
		Updates(map[string]interface{}{"read": true, "read_at": now}).Error
}

func (r *userNotificationRepo) DeleteByUser(ctx context.Context, id int64, userID int64) error {
	return r.GetDB().WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		Delete(&model.UserNotificationModel{}).Error
}

func (r *userNotificationRepo) ClearAllByUser(ctx context.Context, userID int64) (int64, error) {
	var cleared int64
	if err := r.GetDB().WithContext(ctx).Model(&model.UserNotificationModel{}).
		Where("user_id = ?", userID).
		Count(&cleared).Error; err != nil {
		return 0, err
	}
	if err := r.GetDB().WithContext(ctx).
		Where("user_id = ?", userID).
		Delete(&model.UserNotificationModel{}).Error; err != nil {
		return 0, err
	}
	return cleared, nil
}

// ====================================================================================
// 创建消息方法（内部使用）
// ====================================================================================

func (r *userNotificationRepo) CreateNotification(ctx context.Context, userID int64, msgType, title, content string, data map[string]interface{}) error {
	var dataStr string
	if data != nil {
		dataJSON, err := json.Marshal(data)
		if err != nil {
			return err
		}
		dataStr = string(dataJSON)
	}

	notification := &model.UserNotificationModel{
		UserId:    userID,
		Type:      msgType,
		Title:     title,
		Content:   content,
		Data:      dataStr,
		Read:      false,
		Timestamp: time.Now().Unix(),
	}
	return r.GetDB().WithContext(ctx).Create(notification).Error
}

func (r *userNotificationRepo) BatchCreateNotifications(ctx context.Context, userIDs []int64, msgType, title, content string, data map[string]interface{}) error {
	if len(userIDs) == 0 {
		return nil
	}

	var dataStr string
	if data != nil {
		dataJSON, err := json.Marshal(data)
		if err != nil {
			return err
		}
		dataStr = string(dataJSON)
	}

	now := time.Now().Unix()
	notifications := make([]model.UserNotificationModel, len(userIDs))
	for i, userID := range userIDs {
		notifications[i] = model.UserNotificationModel{
			UserId:    userID,
			Type:      msgType,
			Title:     title,
			Content:   content,
			Data:      dataStr,
			Read:      false,
			Timestamp: now,
		}
	}

	return r.GetDB().WithContext(ctx).CreateInBatches(notifications, 100).Error
}
