package repository

import (
	"context"
	"errors"
	"internalwallet/common/constants"
	commonRepo "internalwallet/common/repository"
	"time"

	"gorm.io/gorm"
	"internalwallet/services/signer/rpc/internal/models"
)

// SignatureLogRepository 签名日志数据访问接口
type SignatureLogRepository interface {
	// 继承基础方法（自动获得所有通用 CRUD 方法）
	commonRepo.BaseRepository[models.SignatureLog]

	// 业务查询
	FindByRequestID(ctx context.Context, requestID string) (*models.SignatureLog, error)
	List(ctx context.Context, filter *SignatureLogFilter) ([]*models.SignatureLog, int64, error)
	FindBySeedID(ctx context.Context, seedID int64, limit int) ([]*models.SignatureLog, error)
	FindByChain(ctx context.Context, chain string, startTime, endTime time.Time) ([]*models.SignatureLog, error)

	// 统计查询
	CountByStatus(ctx context.Context, status int) (int64, error)
	CountBySeedAndDateRange(ctx context.Context, seedID int64, startTime, endTime time.Time) (int64, error)

	// 更新操作
	UpdateStatus(ctx context.Context, id int64, status int, errorMsg string) error
	MarkSuccess(ctx context.Context, id int64, txHash string, signedTx string, rawTxHash string, completedAt time.Time) error

	// 金额统计
	SumAmountByDateAndStatus(ctx context.Context, seedID int64, date string, status int) (string, error)
}

// SignatureLogFilter 签名日志查询过滤器
type SignatureLogFilter struct {
	SeedID        *int64
	Chain         string
	OperationType int
	Status        int
	StartDate     *time.Time
	EndDate       *time.Time
	Page          int
	PageSize      int
}

type signatureLogRepo struct {
	commonRepo.BaseRepository[models.SignatureLog]
}

// NewSignatureLogRepository 创建SignatureLog Repository
func NewSignatureLogRepository(db *gorm.DB) SignatureLogRepository {
	return &signatureLogRepo{BaseRepository: commonRepo.NewBaseRepository[models.SignatureLog](db)}
}

// FindByRequestID 根据RequestID查询
func (r *signatureLogRepo) FindByRequestID(ctx context.Context, requestID string) (*models.SignatureLog, error) {
	var log models.SignatureLog
	err := r.GetDB().WithContext(ctx).
		Where("request_id = ?", requestID).
		First(&log).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.New("signature log not found")
	}
	return &log, err
}

// List 分页查询签名日志（支持多条件过滤）
func (r *signatureLogRepo) List(ctx context.Context, filter *SignatureLogFilter) ([]*models.SignatureLog, int64, error) {
	var logs []*models.SignatureLog
	var total int64

	query := r.GetDB().WithContext(ctx).Model(&models.SignatureLog{})

	// 应用过滤条件
	if filter.SeedID != nil && *filter.SeedID > 0 {
		query = query.Where("master_seed_id = ?", *filter.SeedID)
	}
	if filter.Chain != "" {
		query = query.Where("chain = ?", filter.Chain)
	}
	if filter.OperationType > 0 {
		query = query.Where("operation_type = ?", filter.OperationType)
	}
	if filter.Status > 0 {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.StartDate != nil {
		query = query.Where("created_at >= ?", filter.StartDate)
	}
	if filter.EndDate != nil {
		query = query.Where("created_at <= ?", filter.EndDate)
	}

	// 计算总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	offset := (page - 1) * pageSize
	err := query.Offset(offset).
		Limit(pageSize).
		Order("created_at DESC").
		Find(&logs).Error

	return logs, total, err
}

// FindBySeedID 查询指定Seed的最近日志
func (r *signatureLogRepo) FindBySeedID(ctx context.Context, seedID int64, limit int) ([]*models.SignatureLog, error) {
	var logs []*models.SignatureLog
	err := r.GetDB().WithContext(ctx).
		Where("master_seed_id = ?", seedID).
		Order("created_at DESC").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}

// FindByChain 查询指定链在时间范围内的日志
func (r *signatureLogRepo) FindByChain(ctx context.Context, chain string, startTime, endTime time.Time) ([]*models.SignatureLog, error) {
	var logs []*models.SignatureLog
	err := r.GetDB().WithContext(ctx).
		Where("chain = ?", chain).
		Where("created_at BETWEEN ? AND ?", startTime, endTime).
		Order("created_at DESC").
		Find(&logs).Error
	return logs, err
}

// CountByStatus 统计指定状态的日志数量
func (r *signatureLogRepo) CountByStatus(ctx context.Context, status int) (int64, error) {
	var count int64
	err := r.GetDB().WithContext(ctx).
		Model(&models.SignatureLog{}).
		Where("status = ?", status).
		Count(&count).Error
	return count, err
}

// CountBySeedAndDateRange 统计指定Seed在时间范围内的签名次数
func (r *signatureLogRepo) CountBySeedAndDateRange(ctx context.Context, seedID int64, startTime, endTime time.Time) (int64, error) {
	var count int64
	err := r.GetDB().WithContext(ctx).
		Model(&models.SignatureLog{}).
		Where("master_seed_id = ?", seedID).
		Where("created_at BETWEEN ? AND ?", startTime, endTime).
		Count(&count).Error
	return count, err
}

// UpdateStatus 更新签名日志状态
func (r *signatureLogRepo) UpdateStatus(ctx context.Context, id int64, status int, errorMsg string) error {
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now().Local(),
	}
	if errorMsg != "" {
		updates["error_msg"] = errorMsg
	}
	return r.GetDB().WithContext(ctx).
		Model(&models.SignatureLog{}).
		Where("id = ?", id).
		Updates(updates).Error
}

func (r *signatureLogRepo) MarkSuccess(ctx context.Context, id int64, txHash string, signedTx string, rawTxHash string, completedAt time.Time) error {
	return r.GetDB().WithContext(ctx).
		Model(&models.SignatureLog{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":      constants.SignStatusSuccess,
			"tx_hash":     txHash,
			"signed_tx":   signedTx,
			"raw_tx_hash": rawTxHash,
			"updated_at":  completedAt.Local(),
			"error_msg":   nil,
		}).Error
}

// SumAmountByDateAndStatus 统计指定日期和状态的总金额
func (r *signatureLogRepo) SumAmountByDateAndStatus(ctx context.Context, seedID int64, date string, status int) (string, error) {
	var total string
	err := r.GetDB().WithContext(ctx).
		Model(&models.SignatureLog{}).
		Where("master_seed_id = ? AND status = ? AND DATE(created_at) = ?", seedID, status, date).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&total).Error
	return total, err
}
