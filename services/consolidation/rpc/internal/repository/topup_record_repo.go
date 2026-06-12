package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"internalwallet/services/consolidation/rpc/internal/models"

	"gorm.io/gorm"
)

type TopUpRecordRepository interface {
	CreateIgnoreDuplicate(ctx context.Context, rec *models.ConsolidationTopUpRecord) (created bool, err error)
	GetByID(ctx context.Context, id int64) (*models.ConsolidationTopUpRecord, error)
	List(ctx context.Context, req ListTopupRecordsRequest) (records []models.ConsolidationTopUpRecord, total int64, err error)
	ListByTaskID(ctx context.Context, taskID string) ([]models.ConsolidationTopUpRecord, error)
	SoftDeleteByTaskID(ctx context.Context, taskID string) (affected int64, err error)
	GetPendingRecords(ctx context.Context, limit int) ([]models.ConsolidationTopUpRecord, error)
	GetSentRecords(ctx context.Context, limit int) ([]models.ConsolidationTopUpRecord, error)
	MarkSent(ctx context.Context, id int64, txHash string, signedTx string) error
	MarkConfirmed(ctx context.Context, id int64, confirmedAt time.Time) error
	MarkFailed(ctx context.Context, id int64, errorMsg string) error
	IncrementRetry(ctx context.Context, id int64) error
}

type topUpRecordRepo struct {
	db *gorm.DB
}

func NewTopUpRecordRepository(db *gorm.DB) TopUpRecordRepository {
	return &topUpRecordRepo{db: db}
}

func (r *topUpRecordRepo) CreateIgnoreDuplicate(ctx context.Context, rec *models.ConsolidationTopUpRecord) (bool, error) {
	if rec == nil {
		return false, fmt.Errorf("record is nil")
	}
	if r.db == nil {
		return false, fmt.Errorf("db not configured")
	}
	if err := r.db.WithContext(ctx).Create(rec).Error; err != nil {
		if isDuplicateKeyError(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *topUpRecordRepo) GetByID(ctx context.Context, id int64) (*models.ConsolidationTopUpRecord, error) {
	if r.db == nil {
		return nil, fmt.Errorf("db not configured")
	}
	var rec models.ConsolidationTopUpRecord
	if err := r.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&rec).Error; err != nil {
		return nil, err
	}
	return &rec, nil
}

type ListTopupRecordsRequest struct {
	Page          int32
	PageSize      int32
	TaskID        *string
	Chain         *string
	AssetSymbol   *string
	Purpose       *string
	Status        *models.ConsolidationTopUpStatus
	ToAddress     *string
	TxHash        *string
	CreatedAtFrom *time.Time
	CreatedAtTo   *time.Time
}

func (r *topUpRecordRepo) List(ctx context.Context, req ListTopupRecordsRequest) ([]models.ConsolidationTopUpRecord, int64, error) {
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

	q := r.db.WithContext(ctx).Model(&models.ConsolidationTopUpRecord{}).Where("deleted_at IS NULL")
	if req.TaskID != nil {
		taskID := strings.TrimSpace(*req.TaskID)
		if taskID != "" {
			q = q.Where("task_id = ?", taskID)
		}
	}
	if req.Chain != nil {
		chain := strings.TrimSpace(*req.Chain)
		if chain != "" {
			q = q.Where("chain = ?", strings.ToUpper(chain))
		}
	}
	if req.AssetSymbol != nil {
		asset := strings.TrimSpace(*req.AssetSymbol)
		if asset != "" {
			q = q.Where("asset_symbol = ?", strings.ToUpper(asset))
		}
	}
	if req.Purpose != nil {
		purpose := strings.TrimSpace(*req.Purpose)
		if purpose != "" {
			q = q.Where("purpose = ?", purpose)
		}
	}
	if req.Status != nil && *req.Status != 0 {
		q = q.Where("status = ?", *req.Status)
	}
	if req.ToAddress != nil {
		addr := strings.TrimSpace(*req.ToAddress)
		if addr != "" {
			q = q.Where("to_address = ?", addr)
		}
	}
	if req.TxHash != nil {
		txHash := strings.TrimSpace(*req.TxHash)
		if txHash != "" {
			q = q.Where("tx_hash = ?", txHash)
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

	var items []models.ConsolidationTopUpRecord
	offset := int((page - 1) * pageSize)
	if err := q.Order("created_at DESC").Offset(offset).Limit(int(pageSize)).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *topUpRecordRepo) ListByTaskID(ctx context.Context, taskID string) ([]models.ConsolidationTopUpRecord, error) {
	if r.db == nil {
		return nil, fmt.Errorf("db not configured")
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, nil
	}
	var items []models.ConsolidationTopUpRecord
	if err := r.db.WithContext(ctx).
		Where("task_id = ? AND deleted_at IS NULL", taskID).
		Order("created_at ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *topUpRecordRepo) SoftDeleteByTaskID(ctx context.Context, taskID string) (int64, error) {
	if r.db == nil {
		return 0, fmt.Errorf("db not configured")
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return 0, nil
	}
	now := time.Now().Local()
	tx := r.db.WithContext(ctx).Model(&models.ConsolidationTopUpRecord{}).
		Where("task_id = ? AND deleted_at IS NULL", taskID).
		Updates(map[string]any{
			"deleted_at": now,
			"updated_at": now,
		})
	if tx.Error != nil {
		return 0, tx.Error
	}
	return tx.RowsAffected, nil
}

func (r *topUpRecordRepo) GetPendingRecords(ctx context.Context, limit int) ([]models.ConsolidationTopUpRecord, error) {
	if r.db == nil {
		return nil, fmt.Errorf("db not configured")
	}
	if limit <= 0 {
		return nil, nil
	}
	var items []models.ConsolidationTopUpRecord
	if err := r.db.WithContext(ctx).
		Where("deleted_at IS NULL AND status = ?", models.ConsolidationTopUpStatusPending).
		Order("created_at ASC").
		Limit(limit).
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *topUpRecordRepo) GetSentRecords(ctx context.Context, limit int) ([]models.ConsolidationTopUpRecord, error) {
	if r.db == nil {
		return nil, fmt.Errorf("db not configured")
	}
	if limit <= 0 {
		return nil, nil
	}
	var items []models.ConsolidationTopUpRecord
	if err := r.db.WithContext(ctx).
		Where("deleted_at IS NULL AND status = ?", models.ConsolidationTopUpStatusSent).
		Order("created_at ASC").
		Limit(limit).
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *topUpRecordRepo) MarkSent(ctx context.Context, id int64, txHash string, signedTx string) error {
	if r.db == nil {
		return fmt.Errorf("db not configured")
	}
	txHash = strings.TrimSpace(txHash)
	signedTx = strings.TrimSpace(signedTx)
	if txHash == "" {
		return fmt.Errorf("txHash is required")
	}
	if signedTx == "" {
		return fmt.Errorf("signedTx is required")
	}
	now := time.Now().Local()
	return r.db.WithContext(ctx).Model(&models.ConsolidationTopUpRecord{}).
		Where("id = ? AND deleted_at IS NULL AND status = ?", id, models.ConsolidationTopUpStatusPending).
		Updates(map[string]any{
			"status":        models.ConsolidationTopUpStatusSent,
			"tx_hash":       txHash,
			"signed_tx":     signedTx,
			"error_message": nil,
			"updated_at":    now,
		}).Error
}

func (r *topUpRecordRepo) MarkConfirmed(ctx context.Context, id int64, confirmedAt time.Time) error {
	if r.db == nil {
		return fmt.Errorf("db not configured")
	}
	now := time.Now().Local()
	return r.db.WithContext(ctx).Model(&models.ConsolidationTopUpRecord{}).
		Where("id = ? AND deleted_at IS NULL AND status = ?", id, models.ConsolidationTopUpStatusSent).
		Updates(map[string]any{
			"status":        models.ConsolidationTopUpStatusConfirmed,
			"confirmed_at":  confirmedAt,
			"error_message": nil,
			"updated_at":    now,
		}).Error
}

func (r *topUpRecordRepo) MarkFailed(ctx context.Context, id int64, errorMsg string) error {
	if r.db == nil {
		return fmt.Errorf("db not configured")
	}
	errorMsg = strings.TrimSpace(errorMsg)
	now := time.Now().Local()
	return r.db.WithContext(ctx).Model(&models.ConsolidationTopUpRecord{}).
		Where("id = ? AND deleted_at IS NULL AND status IN ?", id, []models.ConsolidationTopUpStatus{
			models.ConsolidationTopUpStatusPending,
			models.ConsolidationTopUpStatusSent,
		}).
		Updates(map[string]any{
			"status":        models.ConsolidationTopUpStatusFailed,
			"error_message": &errorMsg,
			"updated_at":    now,
		}).Error
}

func (r *topUpRecordRepo) IncrementRetry(ctx context.Context, id int64) error {
	if r.db == nil {
		return fmt.Errorf("db not configured")
	}
	now := time.Now().Local()
	return r.db.WithContext(ctx).Model(&models.ConsolidationTopUpRecord{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]any{
			"retry_count": gorm.Expr("retry_count + 1"),
			"updated_at":  now,
		}).Error
}
