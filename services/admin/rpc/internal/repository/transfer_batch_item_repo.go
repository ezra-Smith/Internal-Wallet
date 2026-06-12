package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"internalwallet/services/admin/rpc/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TransferBatchItemFilters struct {
	ID              string
	UID             string
	Nickname        string
	Email           string
	Phone           string
	Role            string
	Currency        string
	Amount          string
	Note            string
	ErrorMessage    string
	ProcessedAtFrom string
	ProcessedAtTo   string
	Status          string
}

type TransferBatchItemRepository interface {
	WithTx(tx *gorm.DB) TransferBatchItemRepository
	GetDB() *gorm.DB

	CreateMany(ctx context.Context, items []*model.TransferBatchItemModel) error
	ListByBatch(ctx context.Context, batchID string, page, pageSize int32, status string) ([]*model.TransferBatchItemModel, int64, error)
	ListByBatchWithFilters(ctx context.Context, batchID string, page, pageSize int32, filters *TransferBatchItemFilters) ([]*model.TransferBatchItemModel, int64, error)
	CountByBatchGroupByStatus(ctx context.Context, batchID string) (map[string]int64, error)
	MarkRetrying(ctx context.Context, batchID string, transferIDs []string) (int64, error)

	// Execution helpers (internal-ledger only)
	ClaimNextRunnable(ctx context.Context, batchID string, now time.Time, minRetryInterval time.Duration) (*model.TransferBatchItemModel, error)
	RecoverStaleProcessing(ctx context.Context, batchID string, now time.Time, staleBefore time.Time, reason string) (int64, error)
	MarkSuccess(ctx context.Context, id string, now time.Time, ledgerTxID *int64) error
	MarkFailed(ctx context.Context, id string, errMsg string, now time.Time) error
}

type transferBatchItemRepo struct {
	db *gorm.DB
}

func NewTransferBatchItemRepository(db *gorm.DB) TransferBatchItemRepository {
	return &transferBatchItemRepo{db: db}
}
func (r *transferBatchItemRepo) WithTx(tx *gorm.DB) TransferBatchItemRepository {
	return &transferBatchItemRepo{db: tx}
}
func (r *transferBatchItemRepo) GetDB() *gorm.DB { return r.db }

func (r *transferBatchItemRepo) CreateMany(ctx context.Context, items []*model.TransferBatchItemModel) error {
	if len(items) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&items).Error
}

func (r *transferBatchItemRepo) ListByBatch(ctx context.Context, batchID string, page, pageSize int32, status string) ([]*model.TransferBatchItemModel, int64, error) {
	batchID = strings.TrimSpace(batchID)
	if batchID == "" {
		return nil, 0, fmt.Errorf("batch not found")
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}

	query := r.db.WithContext(ctx).Model(&model.TransferBatchItemModel{}).Where("batch_id = ?", batchID)
	if strings.TrimSpace(status) != "" {
		query = query.Where("status = ?", strings.TrimSpace(status))
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := int((page - 1) * pageSize)
	var items []*model.TransferBatchItemModel
	err := query.Order("created_at ASC").Offset(offset).Limit(int(pageSize)).Find(&items).Error
	return items, total, err
}

func (r *transferBatchItemRepo) ListByBatchWithFilters(ctx context.Context, batchID string, page, pageSize int32, filters *TransferBatchItemFilters) ([]*model.TransferBatchItemModel, int64, error) {
	batchID = strings.TrimSpace(batchID)
	if batchID == "" {
		return nil, 0, fmt.Errorf("batch not found")
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}

	// Start with base query on transfer_batch_items
	query := r.db.WithContext(ctx).Table("transfer_batch_items i").Where("i.batch_id = ?", batchID)

	// Track if we need to join users table
	needUserJoin := false

	if filters != nil {
		// Filter by transfer_batch_item fields
		if id := strings.TrimSpace(filters.ID); id != "" {
			query = query.Where("i.id LIKE ?", "%"+id+"%")
		}
		if currency := strings.TrimSpace(filters.Currency); currency != "" {
			query = query.Where("i.currency = ?", currency)
		}
		if amount := strings.TrimSpace(filters.Amount); amount != "" {
			query = query.Where("i.amount LIKE ?", "%"+amount+"%")
		}
		if note := strings.TrimSpace(filters.Note); note != "" {
			query = query.Where("i.note LIKE ?", "%"+note+"%")
		}
		if errMsg := strings.TrimSpace(filters.ErrorMessage); errMsg != "" {
			query = query.Where("i.error_message LIKE ?", "%"+errMsg+"%")
		}
		if status := strings.TrimSpace(filters.Status); status != "" {
			query = query.Where("i.status = ?", status)
		}

		// Filter by processed_at date range
		if from := strings.TrimSpace(filters.ProcessedAtFrom); from != "" {
			if t, err := time.Parse(time.RFC3339, from); err == nil {
				query = query.Where("i.processed_at >= ?", t)
			}
		}
		if to := strings.TrimSpace(filters.ProcessedAtTo); to != "" {
			if t, err := time.Parse(time.RFC3339, to); err == nil {
				query = query.Where("i.processed_at <= ?", t)
			}
		}

		// Filter by user fields - need to join users table
		if uid := strings.TrimSpace(filters.UID); uid != "" {
			needUserJoin = true
			query = query.Where("i.user_id = ?", uid)
		}
		if nickname := strings.TrimSpace(filters.Nickname); nickname != "" {
			needUserJoin = true
			query = query.Where("u.nickname LIKE ?", "%"+nickname+"%")
		}
		if email := strings.TrimSpace(filters.Email); email != "" {
			needUserJoin = true
			query = query.Where("u.email LIKE ?", "%"+email+"%")
		}
		if phone := strings.TrimSpace(filters.Phone); phone != "" {
			needUserJoin = true
			// Remove common phone formatting characters for search
			cleanPhone := strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(phone, "-", ""), " ", ""), "+", "")
			query = query.Where("(u.phone LIKE ? OR CONCAT(u.country_code, u.phone) LIKE ?)", "%"+cleanPhone+"%", "%"+cleanPhone+"%")
		}
		if role := strings.TrimSpace(filters.Role); role != "" {
			needUserJoin = true
			// Map role to member_level
			var memberLevel int32
			switch strings.ToLower(role) {
			case "vip":
				memberLevel = 1
			case "svip":
				memberLevel = 2
			case "normal", "regular":
				memberLevel = 0
			default:
				// If not recognized, try to parse as number
				if _, err := fmt.Sscanf(role, "%d", &memberLevel); err != nil {
					memberLevel = -1 // Invalid, won't match anything
				}
			}
			query = query.Where("u.member_level = ?", memberLevel)
		}
	}

	// Add user join if needed
	if needUserJoin {
		query = query.Joins("LEFT JOIN users u ON u.id = i.user_id AND u.deleted_at IS NULL")
	}

	// Count total with filters
	var total int64
	countQuery := query.Session(&gorm.Session{})
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Select only transfer_batch_items columns for the final query
	query = query.Select("i.*")

	// Apply pagination
	offset := int((page - 1) * pageSize)
	var items []*model.TransferBatchItemModel
	err := query.Order("i.created_at ASC").Offset(offset).Limit(int(pageSize)).Scan(&items).Error
	return items, total, err
}

func (r *transferBatchItemRepo) CountByBatchGroupByStatus(ctx context.Context, batchID string) (map[string]int64, error) {
	batchID = strings.TrimSpace(batchID)
	if batchID == "" {
		return nil, fmt.Errorf("batch not found")
	}
	type row struct {
		Status string
		Cnt    int64
	}
	var rows []row
	if err := r.db.WithContext(ctx).
		Model(&model.TransferBatchItemModel{}).
		Select("status as status, COUNT(1) as cnt").
		Where("batch_id = ?", batchID).
		Group("status").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string]int64, len(rows))
	for _, it := range rows {
		out[strings.TrimSpace(it.Status)] = it.Cnt
	}
	return out, nil
}

func (r *transferBatchItemRepo) MarkRetrying(ctx context.Context, batchID string, transferIDs []string) (int64, error) {
	batchID = strings.TrimSpace(batchID)
	if batchID == "" {
		return 0, fmt.Errorf("batch not found")
	}
	q := r.db.WithContext(ctx).Model(&model.TransferBatchItemModel{}).Where("batch_id = ? AND status = ?", batchID, "failed")
	if len(transferIDs) > 0 {
		q = q.Where("id IN ?", transferIDs)
	}
	res := q.Updates(map[string]interface{}{
		"status":      "retrying",
		"retry_count": gorm.Expr("retry_count + 1"),
	})
	if res.Error != nil {
		return 0, res.Error
	}
	return res.RowsAffected, nil
}

var _ TransferBatchItemRepository = (*transferBatchItemRepo)(nil)

var ErrNoRunnableItem = errors.New("no runnable item")

func (r *transferBatchItemRepo) ClaimNextRunnable(ctx context.Context, batchID string, now time.Time, minRetryInterval time.Duration) (*model.TransferBatchItemModel, error) {
	batchID = strings.TrimSpace(batchID)
	if batchID == "" {
		return nil, fmt.Errorf("batch not found")
	}
	if r.db == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	if now.IsZero() {
		now = time.Now()
	}

	var out *model.TransferBatchItemModel
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		q := tx.WithContext(ctx).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Model(&model.TransferBatchItemModel{}).
			Where("batch_id = ? AND status IN ?", batchID, []string{"pending", "retrying"})
		if minRetryInterval > 0 {
			q = q.Where("(processed_at IS NULL OR processed_at < ?)", now.Add(-minRetryInterval))
		}

		var m model.TransferBatchItemModel
		if err := q.Order("created_at ASC").First(&m).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNoRunnableItem
			}
			return err
		}

		if err := tx.WithContext(ctx).
			Model(&model.TransferBatchItemModel{}).
			Where("id = ?", m.ID).
			Updates(map[string]interface{}{
				"status":        "processing",
				"processed_at":  &now,
				"error_message": nil,
			}).Error; err != nil {
			return err
		}

		m.Status = "processing"
		m.ProcessedAt = &now
		m.ErrorMessage = nil
		out = &m
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *transferBatchItemRepo) RecoverStaleProcessing(ctx context.Context, batchID string, now time.Time, staleBefore time.Time, reason string) (int64, error) {
	batchID = strings.TrimSpace(batchID)
	if batchID == "" {
		return 0, fmt.Errorf("batch not found")
	}
	if r.db == nil {
		return 0, fmt.Errorf("db not initialized")
	}
	if now.IsZero() {
		now = time.Now()
	}
	if staleBefore.IsZero() {
		return 0, nil
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "processing expired"
	}

	res := r.db.WithContext(ctx).
		Model(&model.TransferBatchItemModel{}).
		Where("batch_id = ? AND status = ? AND processed_at IS NOT NULL AND processed_at < ?", batchID, "processing", staleBefore).
		Updates(map[string]interface{}{
			"status":        "retrying",
			"processed_at":  &now,
			"error_message": reason,
		})
	if res.Error != nil {
		return 0, res.Error
	}
	return res.RowsAffected, nil
}

func (r *transferBatchItemRepo) MarkSuccess(ctx context.Context, id string, now time.Time, ledgerTxID *int64) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("id required")
	}
	if r.db == nil {
		return fmt.Errorf("db not initialized")
	}
	if now.IsZero() {
		now = time.Now()
	}

	updates := map[string]interface{}{
		"status":        "success",
		"processed_at":  &now,
		"error_message": nil,
	}
	if ledgerTxID != nil && *ledgerTxID > 0 {
		updates["ledger_tx_id"] = *ledgerTxID
	}
	// Idempotent: do not overwrite success with failure states.
	err := r.db.WithContext(ctx).
		Model(&model.TransferBatchItemModel{}).
		Where("id = ? AND status <> ?", id, "success").
		Updates(updates).Error
	if err != nil && ledgerTxID != nil && *ledgerTxID > 0 && isUnknownColumn(err, "ledger_tx_id") {
		// Backward-compatible fallback for local/dev DBs that haven't applied the migration yet.
		delete(updates, "ledger_tx_id")
		return r.db.WithContext(ctx).
			Model(&model.TransferBatchItemModel{}).
			Where("id = ? AND status <> ?", id, "success").
			Updates(updates).Error
	}
	return err
}

func (r *transferBatchItemRepo) MarkFailed(ctx context.Context, id string, errMsg string, now time.Time) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("id required")
	}
	if r.db == nil {
		return fmt.Errorf("db not initialized")
	}
	if now.IsZero() {
		now = time.Now()
	}
	errMsg = strings.TrimSpace(errMsg)
	if errMsg == "" {
		errMsg = "failed"
	}

	updates := map[string]interface{}{
		"status":        "failed",
		"processed_at":  &now,
		"error_message": errMsg,
	}
	// Idempotent: do not overwrite success.
	return r.db.WithContext(ctx).
		Model(&model.TransferBatchItemModel{}).
		Where("id = ? AND status <> ?", id, "success").
		Updates(updates).Error
}

func isUnknownColumn(err error, column string) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	column = strings.ToLower(strings.TrimSpace(column))
	if column == "" {
		return false
	}
	return strings.Contains(msg, "unknown column") && strings.Contains(msg, column)
}

func (r *transferBatchItemRepo) ensureExists(ctx context.Context, batchID string) error {
	var m model.TransferBatchModel
	err := r.db.WithContext(ctx).Where("batch_id = ?", batchID).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("batch not found")
	}
	return err
}
