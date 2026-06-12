package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"internalwallet/services/consolidation/rpc/internal/models"

	"gorm.io/gorm"
)

type ConsolidationLogRepository interface {
	Create(ctx context.Context, logItem *models.ConsolidationLog) error
	List(ctx context.Context, req ListLogsRequest) (logs []models.ConsolidationLog, total int64, err error)
}

type consolidationLogRepo struct {
	db *gorm.DB
}

func NewConsolidationLogRepository(db *gorm.DB) ConsolidationLogRepository {
	return &consolidationLogRepo{db: db}
}

func (r *consolidationLogRepo) Create(ctx context.Context, logItem *models.ConsolidationLog) error {
	if logItem == nil {
		return fmt.Errorf("log item is nil")
	}
	if r.db == nil {
		return fmt.Errorf("db not configured")
	}
	return r.db.WithContext(ctx).Create(logItem).Error
}

type ListLogsRequest struct {
	Page          int32
	PageSize      int32
	TaskID        *string
	LogLevel      *string
	CreatedAtFrom *time.Time
	CreatedAtTo   *time.Time
}

func (r *consolidationLogRepo) List(ctx context.Context, req ListLogsRequest) ([]models.ConsolidationLog, int64, error) {
	if r.db == nil {
		return nil, 0, fmt.Errorf("db not configured")
	}
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}

	q := r.db.WithContext(ctx).Model(&models.ConsolidationLog{}).Where("deleted_at IS NULL")
	if req.TaskID != nil {
		taskID := strings.TrimSpace(*req.TaskID)
		if taskID != "" {
			q = q.Where("task_id = ?", taskID)
		}
	}
	if req.LogLevel != nil {
		level := strings.TrimSpace(*req.LogLevel)
		if level != "" {
			q = q.Where("log_level = ?", strings.ToUpper(level))
		}
	}
	if req.CreatedAtFrom != nil {
		q = q.Where("created_at >= ?", *req.CreatedAtFrom)
	}
	if req.CreatedAtTo != nil {
		q = q.Where("created_at <= ?", *req.CreatedAtTo)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []models.ConsolidationLog
	offset := int((page - 1) * pageSize)
	if err := q.Order("created_at DESC").Offset(offset).Limit(int(pageSize)).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
