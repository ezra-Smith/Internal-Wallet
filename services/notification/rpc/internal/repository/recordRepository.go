package repository

import (
	"context"
	commonRepo "internalwallet/common/repository"
	"internalwallet/services/notification/rpc/internal/model"
	"time"

	"gorm.io/gorm"
)

// RecordRepository 推送记录仓储接口
// 嵌入 BaseRepository，继承基础 CRUD 方法
type RecordRepository interface {
	commonRepo.BaseRepository[model.NotificationRecord]

	// 业务特定方法
	FindByUserID(ctx context.Context, userID int64, notificationType string, page, pageSize int) ([]*model.NotificationRecord, int64, error)
	FindByJPushMsgID(ctx context.Context, jpushMsgID string) (*model.NotificationRecord, error)
	FindPendingRecords(ctx context.Context, limit int) ([]*model.NotificationRecord, error)
	FindFailedRecordsForRetry(ctx context.Context, maxRetries int, limit int) ([]*model.NotificationRecord, error)
	UpdateStatus(ctx context.Context, id int64, status string, errorMsg string) error
	MarkAsSent(ctx context.Context, id int64, jpushMsgID string) error
	MarkAsClicked(ctx context.Context, id int64) error
	IncrementRetryCount(ctx context.Context, id int64) error
	GetStatistics(ctx context.Context, startDate, endDate time.Time, notificationType string) (map[string]int64, error)
}

type recordRepository struct {
	commonRepo.BaseRepository[model.NotificationRecord]
}

// NewRecordRepository 创建推送记录仓储实例
func NewRecordRepository(db *gorm.DB) RecordRepository {
	return &recordRepository{
		BaseRepository: commonRepo.NewBaseRepository[model.NotificationRecord](db),
	}
}

// FindByUserID 根据用户ID查询推送记录（分页）
func (r *recordRepository) FindByUserID(ctx context.Context, userID int64, notificationType string, page, pageSize int) ([]*model.NotificationRecord, int64, error) {
	var records []*model.NotificationRecord
	var total int64

	query := r.GetDB().WithContext(ctx).Model(&model.NotificationRecord{}).Where("user_id = ?", userID)

	// 如果指定了通知类型，添加过滤条件
	if notificationType != "" && notificationType != "unspecified" {
		query = query.Where("notification_type = ?", notificationType)
	}

	// 计算总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	err := query.Order("created_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&records).Error

	return records, total, err
}

// FindByJPushMsgID 根据极光消息ID查询推送记录
func (r *recordRepository) FindByJPushMsgID(ctx context.Context, jpushMsgID string) (*model.NotificationRecord, error) {
	var record model.NotificationRecord
	err := r.GetDB().WithContext(ctx).Where("jpush_msg_id = ?", jpushMsgID).First(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

// FindPendingRecords 查询待发送的推送记录
func (r *recordRepository) FindPendingRecords(ctx context.Context, limit int) ([]*model.NotificationRecord, error) {
	var records []*model.NotificationRecord
	err := r.GetDB().WithContext(ctx).
		Where("push_status = ?", "pending").
		Order("created_at ASC").
		Limit(limit).
		Find(&records).Error
	return records, err
}

// FindFailedRecordsForRetry 查询需要重试的失败记录
func (r *recordRepository) FindFailedRecordsForRetry(ctx context.Context, maxRetries int, limit int) ([]*model.NotificationRecord, error) {
	var records []*model.NotificationRecord
	err := r.GetDB().WithContext(ctx).
		Where("push_status = ?", "failed").
		Where("retry_count < ?", maxRetries).
		Order("created_at ASC").
		Limit(limit).
		Find(&records).Error
	return records, err
}

// UpdateStatus 更新推送状态
func (r *recordRepository) UpdateStatus(ctx context.Context, id int64, status string, errorMsg string) error {
	updates := map[string]interface{}{
		"push_status": status,
	}
	if errorMsg != "" {
		updates["error_message"] = errorMsg
	}
	return r.GetDB().WithContext(ctx).
		Model(&model.NotificationRecord{}).
		Where("id = ?", id).
		Updates(updates).
		Error
}

// MarkAsSent 标记为已发送
func (r *recordRepository) MarkAsSent(ctx context.Context, id int64, jpushMsgID string) error {
	now := time.Now()
	return r.GetDB().WithContext(ctx).
		Model(&model.NotificationRecord{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"push_status":   "sent",
			"jpush_msg_id":  jpushMsgID,
			"sent_at":       now,
			"error_message": "", // 清空错误信息
		}).
		Error
}

// MarkAsClicked 标记为已点击
func (r *recordRepository) MarkAsClicked(ctx context.Context, id int64) error {
	now := time.Now()
	return r.GetDB().WithContext(ctx).
		Model(&model.NotificationRecord{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"push_status": "clicked",
			"clicked_at":  now,
		}).
		Error
}

// IncrementRetryCount 增加重试次数
func (r *recordRepository) IncrementRetryCount(ctx context.Context, id int64) error {
	return r.GetDB().WithContext(ctx).
		Model(&model.NotificationRecord{}).
		Where("id = ?", id).
		Update("retry_count", gorm.Expr("retry_count + 1")).
		Error
}

// GetStatistics 获取统计数据
func (r *recordRepository) GetStatistics(ctx context.Context, startDate, endDate time.Time, notificationType string) (map[string]int64, error) {
	stats := make(map[string]int64)

	query := r.GetDB().WithContext(ctx).Model(&model.NotificationRecord{}).
		Where("created_at >= ? AND created_at <= ?", startDate, endDate)

	if notificationType != "" && notificationType != "unspecified" {
		query = query.Where("notification_type = ?", notificationType)
	}

	// 总发送数
	var totalSent int64
	if err := query.Count(&totalSent).Error; err != nil {
		return nil, err
	}
	stats["total_sent"] = totalSent

	// 成功数
	var totalSuccess int64
	if err := query.Where("push_status = ?", "sent").Or("push_status = ?", "clicked").Count(&totalSuccess).Error; err != nil {
		return nil, err
	}
	stats["total_success"] = totalSuccess

	// 失败数
	var totalFailed int64
	if err := query.Where("push_status = ?", "failed").Count(&totalFailed).Error; err != nil {
		return nil, err
	}
	stats["total_failed"] = totalFailed

	// 点击数
	var totalClicked int64
	if err := query.Where("push_status = ?", "clicked").Count(&totalClicked).Error; err != nil {
		return nil, err
	}
	stats["total_clicked"] = totalClicked

	return stats, nil
}
