package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"internalwallet/services/consolidation/rpc/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ConsolidationTaskRepository interface {
	CreateIgnoreDuplicate(ctx context.Context, task *models.ConsolidationTask) (created bool, err error)
	GetByTaskID(ctx context.Context, taskID string) (*models.ConsolidationTask, error)
	List(ctx context.Context, req ListTasksRequest) (tasks []models.ConsolidationTask, total int64, err error)
	GetStats(ctx context.Context, req StatsRequest) (*StatsResult, error)

	// Leasing / claiming (multi-instance safe).
	ClaimByStatuses(ctx context.Context, statuses []models.ConsolidationTaskStatus, chain string, limit int, now time.Time, lease time.Duration, instanceID string) ([]models.ConsolidationTask, error)
	ClaimByTaskID(ctx context.Context, taskID string, now time.Time, lease time.Duration, instanceID string) (task *models.ConsolidationTask, ok bool, err error)
	RenewClaim(ctx context.Context, id int64, instanceID string, now time.Time, lease time.Duration) (ok bool, err error)
	ReleaseClaim(ctx context.Context, id int64, instanceID string) error

	// Worker-owned state transitions (require valid claim + expected version).
	MarkDeferredPending(ctx context.Context, id int64, instanceID string, expectedVersion int64, msg string, nextAttemptAt *time.Time) (ok bool, err error)
	MarkNeedEnergy(ctx context.Context, id int64, instanceID string, expectedVersion int64, msg string, nextAttemptAt *time.Time) (ok bool, err error)
	MarkNeedBandwidth(ctx context.Context, id int64, instanceID string, expectedVersion int64, msg string, nextAttemptAt *time.Time) (ok bool, err error)
	MarkNeedGas(ctx context.Context, id int64, instanceID string, expectedVersion int64, msg string, nextAttemptAt *time.Time) (ok bool, err error)
	MarkInProgress(ctx context.Context, id int64, instanceID string, expectedVersion int64, txHash string, signedTx string, startedAt time.Time, estimatedFee *string, amount string) (ok bool, err error)
	MarkRebroadcastInProgress(ctx context.Context, id int64, instanceID string, expectedVersion int64, txHash string, signedTx string, startedAt time.Time, estimatedFee *string, amount string, msg string, bumpCount int32) (ok bool, err error)
	MarkTimeout(ctx context.Context, id int64, instanceID string, expectedVersion int64, msg string) (ok bool, err error)
	MarkConfirmed(ctx context.Context, id int64, instanceID string, expectedVersion int64, confirmedAt time.Time, actualFee *string) (ok bool, err error)
	MarkRetryPending(ctx context.Context, id int64, instanceID string, expectedVersion int64, msg string, nextAttemptAt *time.Time, retryCount int32) (ok bool, err error)
	MarkFailed(ctx context.Context, id int64, instanceID string, expectedVersion int64, status models.ConsolidationTaskStatus, msg string, nextAttemptAt *time.Time, retryCount int32) (ok bool, err error)
	MarkPending(ctx context.Context, id int64, instanceID string, expectedVersion int64, nextAttemptAt *time.Time) (ok bool, err error)
	AttachEnergyRental(ctx context.Context, id int64, instanceID string, expectedVersion int64, energyRentalID int64, totalCostSun int64) (ok bool, err error)
	ClearEnergyRental(ctx context.Context, id int64, instanceID string, expectedVersion int64) (ok bool, err error)

	HasRecentConfirmed(ctx context.Context, chain, assetSymbol, fromAddress string, tokenContract *string, since time.Time) (bool, error)
	HasActiveTask(ctx context.Context, chain, fromAddress string) (bool, error)
	HasTerminalFailedTask(ctx context.Context, chain, fromAddress string) (bool, error)
	HasTerminalFailedTokenTask(ctx context.Context, chain, fromAddress string) (bool, error)

	// Admin/manual transitions (no claim required; still bumps version).
	MarkCancelled(ctx context.Context, id int64) (ok bool, err error)
	ForceRetryPending(ctx context.Context, id int64, msg string, nextAttemptAt *time.Time, retryCount int32) (ok bool, err error)
}

type consolidationTaskRepo struct {
	db *gorm.DB
}

func NewConsolidationTaskRepository(db *gorm.DB) ConsolidationTaskRepository {
	return &consolidationTaskRepo{db: db}
}

func (r *consolidationTaskRepo) CreateIgnoreDuplicate(ctx context.Context, task *models.ConsolidationTask) (bool, error) {
	if task == nil {
		return false, fmt.Errorf("task is nil")
	}
	if r.db == nil {
		return false, fmt.Errorf("db not configured")
	}
	if err := r.db.WithContext(ctx).Create(task).Error; err != nil {
		if isDuplicateKeyError(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *consolidationTaskRepo) GetByTaskID(ctx context.Context, taskID string) (*models.ConsolidationTask, error) {
	if r.db == nil {
		return nil, fmt.Errorf("db not configured")
	}
	var task models.ConsolidationTask
	if err := r.db.WithContext(ctx).Where("task_id = ? AND deleted_at IS NULL", taskID).First(&task).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

type ListTasksRequest struct {
	Page          int32
	PageSize      int32
	Chain         *string
	Asset         *string
	Status        *models.ConsolidationTaskStatus
	CreatedAtFrom *time.Time
	CreatedAtTo   *time.Time
	TaskID        *string
	FromAddress   *string
	ToAddress     *string
	TxHash        *string
}

func (r *consolidationTaskRepo) List(ctx context.Context, req ListTasksRequest) ([]models.ConsolidationTask, int64, error) {
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

	q := r.db.WithContext(ctx).Model(&models.ConsolidationTask{}).Where("deleted_at IS NULL")
	if req.Chain != nil && *req.Chain != "" {
		q = q.Where("chain = ?", strings.ToUpper(strings.TrimSpace(*req.Chain)))
	}
	if req.Asset != nil && *req.Asset != "" {
		q = q.Where("asset_symbol = ?", strings.ToUpper(strings.TrimSpace(*req.Asset)))
	}
	if req.Status != nil && *req.Status != 0 {
		q = q.Where("status = ?", *req.Status)
	}
	if req.CreatedAtFrom != nil {
		q = q.Where("created_at >= ?", *req.CreatedAtFrom)
	}
	if req.CreatedAtTo != nil {
		q = q.Where("created_at <= ?", *req.CreatedAtTo)
	}
	if req.TaskID != nil && strings.TrimSpace(*req.TaskID) != "" {
		q = q.Where("task_id = ?", strings.TrimSpace(*req.TaskID))
	}
	if req.FromAddress != nil && strings.TrimSpace(*req.FromAddress) != "" {
		q = q.Where("from_address = ?", strings.TrimSpace(*req.FromAddress))
	}
	if req.ToAddress != nil && strings.TrimSpace(*req.ToAddress) != "" {
		q = q.Where("to_address = ?", strings.TrimSpace(*req.ToAddress))
	}
	if req.TxHash != nil && strings.TrimSpace(*req.TxHash) != "" {
		q = q.Where("tx_hash = ?", strings.TrimSpace(*req.TxHash))
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var tasks []models.ConsolidationTask
	offset := int((page - 1) * pageSize)
	if err := q.Order("created_at DESC").Offset(offset).Limit(int(pageSize)).Find(&tasks).Error; err != nil {
		return nil, 0, err
	}
	return tasks, total, nil
}

type StatsRequest struct {
	Chain *string
	Asset *string
	From  *time.Time
	To    *time.Time
}

type StatsResult struct {
	Total             int64
	Confirmed         int64
	Failed            int64
	TotalAmount       string
	TotalEstimatedFee string
	TotalActualFee    string
}

func (r *consolidationTaskRepo) GetStats(ctx context.Context, req StatsRequest) (*StatsResult, error) {
	if r.db == nil {
		return nil, fmt.Errorf("db not configured")
	}
	buildBase := func() *gorm.DB {
		q := r.db.WithContext(ctx).Model(&models.ConsolidationTask{}).Where("deleted_at IS NULL")
		if req.Chain != nil && *req.Chain != "" {
			q = q.Where("chain = ?", strings.ToUpper(strings.TrimSpace(*req.Chain)))
		}
		if req.Asset != nil && *req.Asset != "" {
			q = q.Where("asset_symbol = ?", strings.ToUpper(strings.TrimSpace(*req.Asset)))
		}
		if req.From != nil {
			q = q.Where("created_at >= ?", *req.From)
		}
		if req.To != nil {
			q = q.Where("created_at <= ?", *req.To)
		}
		return q
	}

	var total int64
	if err := buildBase().Count(&total).Error; err != nil {
		return nil, err
	}

	var confirmed int64
	if err := buildBase().Where("status = ?", models.ConsolidationTaskStatusConfirmed).Count(&confirmed).Error; err != nil {
		return nil, err
	}

	var failed int64
	if err := buildBase().Where("status IN ?", []models.ConsolidationTaskStatus{
		models.ConsolidationTaskStatusFailed,
		models.ConsolidationTaskStatusTimeout,
		models.ConsolidationTaskStatusPermanentFailed,
	}).Count(&failed).Error; err != nil {
		return nil, err
	}

	// Amount/fee totals are strings; for Phase 1, keep them empty (aggregation by bigint parsing can be added later).
	return &StatsResult{
		Total:     total,
		Confirmed: confirmed,
		Failed:    failed,
	}, nil
}

func (r *consolidationTaskRepo) ClaimByStatuses(ctx context.Context, statuses []models.ConsolidationTaskStatus, chain string, limit int, now time.Time, lease time.Duration, instanceID string) ([]models.ConsolidationTask, error) {
	if r.db == nil {
		return nil, fmt.Errorf("db not configured")
	}
	if limit <= 0 || len(statuses) == 0 {
		return nil, nil
	}
	if lease <= 0 {
		lease = 60 * time.Second
	}
	leaseSeconds := int64(lease.Seconds())
	if leaseSeconds <= 0 {
		leaseSeconds = 60
	}
	chain = strings.ToUpper(strings.TrimSpace(chain))

	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback().Error
			panic(p)
		}
	}()

	var ids []int64
	q := tx.Model(&models.ConsolidationTask{}).
		Where("deleted_at IS NULL").
		Where("status IN ?", statuses).
		Where("(claimed_until IS NULL OR claimed_until < NOW())").
		Where("(next_attempt_at IS NULL OR next_attempt_at <= NOW())")
	if chain != "" {
		q = q.Where("chain = ?", chain)
	}
	if err := q.
		Order("created_at ASC").
		Limit(limit).
		Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
		Pluck("id", &ids).Error; err != nil {
		_ = tx.Rollback().Error
		return nil, err
	}
	if len(ids) == 0 {
		_ = tx.Commit().Error
		return nil, nil
	}

	if err := tx.Model(&models.ConsolidationTask{}).
		Where("id IN ?", ids).
		Updates(map[string]any{
			"claimed_by":    instanceID,
			"claimed_until": gorm.Expr("DATE_ADD(NOW(), INTERVAL ? SECOND)", leaseSeconds),
			"updated_at":    gorm.Expr("NOW()"),
		}).Error; err != nil {
		_ = tx.Rollback().Error
		return nil, err
	}

	var tasks []models.ConsolidationTask
	if err := tx.Where("id IN ?", ids).Find(&tasks).Error; err != nil {
		_ = tx.Rollback().Error
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}
	return tasks, nil
}

func (r *consolidationTaskRepo) ClaimByTaskID(ctx context.Context, taskID string, now time.Time, lease time.Duration, instanceID string) (*models.ConsolidationTask, bool, error) {
	if r.db == nil {
		return nil, false, fmt.Errorf("db not configured")
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, false, nil
	}
	if lease <= 0 {
		lease = 60 * time.Second
	}
	leaseSeconds := int64(lease.Seconds())
	if leaseSeconds <= 0 {
		leaseSeconds = 60
	}

	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, false, tx.Error
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback().Error
			panic(p)
		}
	}()

	var id int64
	q := tx.Model(&models.ConsolidationTask{}).
		Where("deleted_at IS NULL").
		Where("task_id = ?", taskID).
		Where("(claimed_until IS NULL OR claimed_until < NOW())")
	if err := q.
		Limit(1).
		Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
		Pluck("id", &id).Error; err != nil {
		_ = tx.Rollback().Error
		return nil, false, err
	}
	if id == 0 {
		_ = tx.Commit().Error
		return nil, false, nil
	}

	if err := tx.Model(&models.ConsolidationTask{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"claimed_by":    instanceID,
			"claimed_until": gorm.Expr("DATE_ADD(NOW(), INTERVAL ? SECOND)", leaseSeconds),
			"updated_at":    gorm.Expr("NOW()"),
		}).Error; err != nil {
		_ = tx.Rollback().Error
		return nil, false, err
	}

	var task models.ConsolidationTask
	if err := tx.Where("id = ?", id).First(&task).Error; err != nil {
		_ = tx.Rollback().Error
		return nil, false, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, false, err
	}
	return &task, true, nil
}

func (r *consolidationTaskRepo) RenewClaim(ctx context.Context, id int64, instanceID string, now time.Time, lease time.Duration) (bool, error) {
	if r.db == nil {
		return false, fmt.Errorf("db not configured")
	}
	if lease <= 0 {
		lease = 60 * time.Second
	}
	leaseSeconds := int64(lease.Seconds())
	if leaseSeconds <= 0 {
		leaseSeconds = 60
	}
	tx := r.db.WithContext(ctx).Model(&models.ConsolidationTask{}).
		Where("id = ? AND deleted_at IS NULL AND claimed_by = ? AND claimed_until IS NOT NULL AND claimed_until >= NOW()", id, instanceID).
		Updates(map[string]any{
			"claimed_until": gorm.Expr("DATE_ADD(NOW(), INTERVAL ? SECOND)", leaseSeconds),
			"updated_at":    gorm.Expr("NOW()"),
		})
	if tx.Error != nil {
		return false, tx.Error
	}
	return tx.RowsAffected > 0, nil
}

func (r *consolidationTaskRepo) ReleaseClaim(ctx context.Context, id int64, instanceID string) error {
	if r.db == nil {
		return fmt.Errorf("db not configured")
	}
	return r.db.WithContext(ctx).Model(&models.ConsolidationTask{}).
		Where("id = ? AND deleted_at IS NULL AND claimed_by = ?", id, instanceID).
		Updates(map[string]any{
			"claimed_by":    nil,
			"claimed_until": nil,
			"updated_at":    gorm.Expr("NOW()"),
		}).Error
}

func (r *consolidationTaskRepo) updateWithLease(ctx context.Context, id int64, instanceID string, expectedVersion int64, fromStatuses []models.ConsolidationTaskStatus, updates map[string]any) (bool, error) {
	if r.db == nil {
		return false, fmt.Errorf("db not configured")
	}
	q := r.db.WithContext(ctx).Model(&models.ConsolidationTask{}).
		Where("id = ? AND deleted_at IS NULL AND version = ?", id, expectedVersion).
		Where("claimed_by = ? AND claimed_until IS NOT NULL AND claimed_until >= NOW()", instanceID)
	if len(fromStatuses) > 0 {
		q = q.Where("status IN ?", fromStatuses)
	}
	updates["updated_at"] = gorm.Expr("NOW()")
	updates["version"] = gorm.Expr("version + 1")

	tx := q.Updates(updates)
	if tx.Error != nil {
		return false, tx.Error
	}
	return tx.RowsAffected > 0, nil
}

func (r *consolidationTaskRepo) MarkDeferredPending(ctx context.Context, id int64, instanceID string, expectedVersion int64, msg string, nextAttemptAt *time.Time) (bool, error) {
	errorMsg := strings.TrimSpace(msg)
	return r.updateWithLease(ctx, id, instanceID, expectedVersion, []models.ConsolidationTaskStatus{models.ConsolidationTaskStatusPending}, map[string]any{
		"status":          models.ConsolidationTaskStatusPending,
		"error_message":   &errorMsg,
		"next_attempt_at": nextAttemptAt,
		"claimed_by":      nil,
		"claimed_until":   nil,
	})
}

func (r *consolidationTaskRepo) MarkNeedEnergy(ctx context.Context, id int64, instanceID string, expectedVersion int64, msg string, nextAttemptAt *time.Time) (bool, error) {
	errorMsg := strings.TrimSpace(msg)
	return r.updateWithLease(ctx, id, instanceID, expectedVersion, []models.ConsolidationTaskStatus{models.ConsolidationTaskStatusPending, models.ConsolidationTaskStatusNeedEnergy, models.ConsolidationTaskStatusNeedBandwidth}, map[string]any{
		"status":          models.ConsolidationTaskStatusNeedEnergy,
		"error_message":   &errorMsg,
		"next_attempt_at": nextAttemptAt,
		"claimed_by":      nil,
		"claimed_until":   nil,
	})
}

func (r *consolidationTaskRepo) MarkNeedBandwidth(ctx context.Context, id int64, instanceID string, expectedVersion int64, msg string, nextAttemptAt *time.Time) (bool, error) {
	errorMsg := strings.TrimSpace(msg)
	return r.updateWithLease(ctx, id, instanceID, expectedVersion, []models.ConsolidationTaskStatus{models.ConsolidationTaskStatusPending, models.ConsolidationTaskStatusNeedBandwidth, models.ConsolidationTaskStatusNeedEnergy}, map[string]any{
		"status":          models.ConsolidationTaskStatusNeedBandwidth,
		"error_message":   &errorMsg,
		"next_attempt_at": nextAttemptAt,
		"claimed_by":      nil,
		"claimed_until":   nil,
	})
}

func (r *consolidationTaskRepo) MarkNeedGas(ctx context.Context, id int64, instanceID string, expectedVersion int64, msg string, nextAttemptAt *time.Time) (bool, error) {
	errorMsg := strings.TrimSpace(msg)
	return r.updateWithLease(ctx, id, instanceID, expectedVersion, []models.ConsolidationTaskStatus{models.ConsolidationTaskStatusPending, models.ConsolidationTaskStatusNeedGas, models.ConsolidationTaskStatusNeedEnergy}, map[string]any{
		"status":          models.ConsolidationTaskStatusNeedGas,
		"error_message":   &errorMsg,
		"next_attempt_at": nextAttemptAt,
		"claimed_by":      nil,
		"claimed_until":   nil,
	})
}

func (r *consolidationTaskRepo) MarkInProgress(ctx context.Context, id int64, instanceID string, expectedVersion int64, txHash string, signedTx string, startedAt time.Time, estimatedFee *string, amount string) (bool, error) {
	amount = strings.TrimSpace(amount)
	if amount == "" {
		return false, fmt.Errorf("amount is required")
	}
	return r.updateWithLease(ctx, id, instanceID, expectedVersion, []models.ConsolidationTaskStatus{models.ConsolidationTaskStatusPending}, map[string]any{
		"status":          models.ConsolidationTaskStatusInProgress,
		"tx_hash":         strings.TrimSpace(txHash),
		"signed_tx":       strings.TrimSpace(signedTx),
		"started_at":      startedAt,
		"estimated_fee":   estimatedFee,
		"amount":          amount,
		"error_message":   nil,
		"next_attempt_at": nil,
	})
}

func (r *consolidationTaskRepo) MarkRebroadcastInProgress(ctx context.Context, id int64, instanceID string, expectedVersion int64, txHash string, signedTx string, startedAt time.Time, estimatedFee *string, amount string, msg string, bumpCount int32) (bool, error) {
	errorMsg := strings.TrimSpace(msg)
	amount = strings.TrimSpace(amount)
	if amount == "" {
		return false, fmt.Errorf("amount is required")
	}
	return r.updateWithLease(ctx, id, instanceID, expectedVersion, []models.ConsolidationTaskStatus{models.ConsolidationTaskStatusInProgress, models.ConsolidationTaskStatusTimeout}, map[string]any{
		"status":          models.ConsolidationTaskStatusInProgress,
		"tx_hash":         strings.TrimSpace(txHash),
		"signed_tx":       strings.TrimSpace(signedTx),
		"started_at":      startedAt,
		"estimated_fee":   estimatedFee,
		"amount":          amount,
		"bump_count":      bumpCount,
		"error_message":   &errorMsg,
		"next_attempt_at": nil,
	})
}

func (r *consolidationTaskRepo) MarkTimeout(ctx context.Context, id int64, instanceID string, expectedVersion int64, msg string) (bool, error) {
	errorMsg := strings.TrimSpace(msg)
	return r.updateWithLease(ctx, id, instanceID, expectedVersion, []models.ConsolidationTaskStatus{models.ConsolidationTaskStatusInProgress}, map[string]any{
		"status":        models.ConsolidationTaskStatusTimeout,
		"error_message": &errorMsg,
		"claimed_by":    nil,
		"claimed_until": nil,
	})
}

func (r *consolidationTaskRepo) MarkConfirmed(ctx context.Context, id int64, instanceID string, expectedVersion int64, confirmedAt time.Time, actualFee *string) (bool, error) {
	return r.updateWithLease(ctx, id, instanceID, expectedVersion, []models.ConsolidationTaskStatus{models.ConsolidationTaskStatusInProgress, models.ConsolidationTaskStatusTimeout}, map[string]any{
		"status":          models.ConsolidationTaskStatusConfirmed,
		"confirmed_at":    confirmedAt,
		"actual_fee":      actualFee,
		"next_attempt_at": nil,
		"claimed_by":      nil,
		"claimed_until":   nil,
	})
}

func (r *consolidationTaskRepo) MarkRetryPending(ctx context.Context, id int64, instanceID string, expectedVersion int64, msg string, nextAttemptAt *time.Time, retryCount int32) (bool, error) {
	errorMsg := strings.TrimSpace(msg)
	return r.updateWithLease(ctx, id, instanceID, expectedVersion, []models.ConsolidationTaskStatus{models.ConsolidationTaskStatusInProgress, models.ConsolidationTaskStatusTimeout}, map[string]any{
		"status":          models.ConsolidationTaskStatusPending,
		"error_message":   &errorMsg,
		"next_attempt_at": nextAttemptAt,
		"retry_count":     retryCount,
		"bump_count":      int32(0),
		"tx_hash":         nil,
		"signed_tx":       nil,
		"started_at":      nil,
		"claimed_by":      nil,
		"claimed_until":   nil,
	})
}

func (r *consolidationTaskRepo) MarkFailed(ctx context.Context, id int64, instanceID string, expectedVersion int64, status models.ConsolidationTaskStatus, msg string, nextAttemptAt *time.Time, retryCount int32) (bool, error) {
	errorMsg := strings.TrimSpace(msg)
	return r.updateWithLease(ctx, id, instanceID, expectedVersion, nil, map[string]any{
		"status":          status,
		"error_message":   &errorMsg,
		"next_attempt_at": nextAttemptAt,
		"retry_count":     retryCount,
		"claimed_by":      nil,
		"claimed_until":   nil,
	})
}

func (r *consolidationTaskRepo) MarkPending(ctx context.Context, id int64, instanceID string, expectedVersion int64, nextAttemptAt *time.Time) (bool, error) {
	return r.updateWithLease(ctx, id, instanceID, expectedVersion, nil, map[string]any{
		"status":          models.ConsolidationTaskStatusPending,
		"next_attempt_at": nextAttemptAt,
		"error_message":   nil,
		"bump_count":      int32(0),
		"tx_hash":         nil,
		"signed_tx":       nil,
		"started_at":      nil,
		"claimed_by":      nil,
		"claimed_until":   nil,
	})
}

func (r *consolidationTaskRepo) AttachEnergyRental(ctx context.Context, id int64, instanceID string, expectedVersion int64, energyRentalID int64, totalCostSun int64) (bool, error) {
	if energyRentalID <= 0 {
		return false, fmt.Errorf("energyRentalID is required")
	}
	if totalCostSun < 0 {
		return false, fmt.Errorf("totalCostSun must be >= 0")
	}
	if r.db == nil {
		return false, fmt.Errorf("db not configured")
	}
	tx := r.db.WithContext(ctx).Model(&models.ConsolidationTask{}).
		Where("id = ? AND deleted_at IS NULL AND version = ?", id, expectedVersion).
		Where("status = ?", models.ConsolidationTaskStatusNeedEnergy).
		Where("claimed_by = ? AND claimed_until IS NOT NULL AND claimed_until >= NOW()", instanceID).
		Where("energy_rental_id IS NULL OR energy_rental_id = 0").
		Updates(map[string]any{
			"energy_rental_id":      energyRentalID,
			"updated_at":            gorm.Expr("NOW()"),
			"version":               gorm.Expr("version + 1"),
			"energy_order_count":    gorm.Expr("energy_order_count + 1"),
			"energy_total_cost_sun": gorm.Expr("energy_total_cost_sun + ?", totalCostSun),
		})
	if tx.Error != nil {
		return false, tx.Error
	}
	return tx.RowsAffected > 0, nil
}

func (r *consolidationTaskRepo) ClearEnergyRental(ctx context.Context, id int64, instanceID string, expectedVersion int64) (bool, error) {
	if r.db == nil {
		return false, fmt.Errorf("db not configured")
	}
	return r.updateWithLease(ctx, id, instanceID, expectedVersion, []models.ConsolidationTaskStatus{models.ConsolidationTaskStatusNeedEnergy}, map[string]any{
		"energy_rental_id": nil,
	})
}

func (r *consolidationTaskRepo) HasRecentConfirmed(ctx context.Context, chain, assetSymbol, fromAddress string, tokenContract *string, since time.Time) (bool, error) {
	if r.db == nil {
		return false, fmt.Errorf("db not configured")
	}
	chain = strings.ToUpper(strings.TrimSpace(chain))
	assetSymbol = strings.ToUpper(strings.TrimSpace(assetSymbol))
	fromAddress = normalizeFromAddress(chain, fromAddress)

	q := r.db.WithContext(ctx).Model(&models.ConsolidationTask{}).
		Where("deleted_at IS NULL AND status = ? AND chain = ? AND asset_symbol = ? AND from_address = ? AND confirmed_at >= ?",
			models.ConsolidationTaskStatusConfirmed, chain, assetSymbol, fromAddress, since)
	if tokenContract == nil || strings.TrimSpace(*tokenContract) == "" {
		q = q.Where("token_contract IS NULL OR token_contract = ''")
	} else {
		contract := normalizeTokenContract(chain, *tokenContract)
		q = q.Where("token_contract = ?", contract)
	}
	var cnt int64
	if err := q.Count(&cnt).Error; err != nil {
		return false, err
	}
	return cnt > 0, nil
}

func (r *consolidationTaskRepo) HasActiveTask(ctx context.Context, chain, fromAddress string) (bool, error) {
	if r.db == nil {
		return false, fmt.Errorf("db not configured")
	}
	chain = strings.ToUpper(strings.TrimSpace(chain))
	fromAddress = normalizeFromAddress(chain, fromAddress)
	activeKey := fmt.Sprintf("%s#%s", chain, fromAddress)
	q := r.db.WithContext(ctx).Model(&models.ConsolidationTask{}).Where("active_task_key = ?", activeKey)
	var cnt int64
	if err := q.Count(&cnt).Error; err != nil {
		return false, err
	}
	return cnt > 0, nil
}

func (r *consolidationTaskRepo) HasTerminalFailedTask(ctx context.Context, chain, fromAddress string) (bool, error) {
	if r.db == nil {
		return false, fmt.Errorf("db not configured")
	}
	chain = strings.ToUpper(strings.TrimSpace(chain))
	fromAddress = normalizeFromAddress(chain, fromAddress)
	var cnt int64
	if err := r.db.WithContext(ctx).
		Model(&models.ConsolidationTask{}).
		Where("deleted_at IS NULL").
		Where("chain = ? AND from_address = ?", chain, fromAddress).
		Where("status IN ?", []models.ConsolidationTaskStatus{
			models.ConsolidationTaskStatusFailed,
			models.ConsolidationTaskStatusPermanentFailed,
		}).
		Count(&cnt).Error; err != nil {
		return false, err
	}
	return cnt > 0, nil
}

func (r *consolidationTaskRepo) HasTerminalFailedTokenTask(ctx context.Context, chain, fromAddress string) (bool, error) {
	if r.db == nil {
		return false, fmt.Errorf("db not configured")
	}
	chain = strings.ToUpper(strings.TrimSpace(chain))
	fromAddress = normalizeFromAddress(chain, fromAddress)
	var cnt int64
	if err := r.db.WithContext(ctx).
		Model(&models.ConsolidationTask{}).
		Where("deleted_at IS NULL").
		Where("chain = ? AND from_address = ?", chain, fromAddress).
		Where("token_contract IS NOT NULL AND token_contract <> ''").
		Where("status IN ?", []models.ConsolidationTaskStatus{
			models.ConsolidationTaskStatusFailed,
			models.ConsolidationTaskStatusPermanentFailed,
		}).
		Count(&cnt).Error; err != nil {
		return false, err
	}
	return cnt > 0, nil
}

func (r *consolidationTaskRepo) MarkCancelled(ctx context.Context, id int64) (bool, error) {
	if r.db == nil {
		return false, fmt.Errorf("db not configured")
	}
	now := time.Now().Local()
	tx := r.db.WithContext(ctx).Model(&models.ConsolidationTask{}).
		Where("id = ? AND deleted_at IS NULL AND status IN ?", id, []models.ConsolidationTaskStatus{
			models.ConsolidationTaskStatusPending,
			models.ConsolidationTaskStatusNeedEnergy,
			models.ConsolidationTaskStatusNeedBandwidth,
			models.ConsolidationTaskStatusNeedGas,
			models.ConsolidationTaskStatusTimeout,
		}).
		Updates(map[string]any{
			"status":        models.ConsolidationTaskStatusCancelled,
			"claimed_by":    nil,
			"claimed_until": nil,
			"updated_at":    now,
			"version":       gorm.Expr("version + 1"),
		})
	if tx.Error != nil {
		return false, tx.Error
	}
	return tx.RowsAffected > 0, nil
}

func (r *consolidationTaskRepo) ForceRetryPending(ctx context.Context, id int64, msg string, nextAttemptAt *time.Time, retryCount int32) (bool, error) {
	if r.db == nil {
		return false, fmt.Errorf("db not configured")
	}
	now := time.Now().Local()
	errorMsg := strings.TrimSpace(msg)
	tx := r.db.WithContext(ctx).Model(&models.ConsolidationTask{}).
		Where("id = ? AND deleted_at IS NULL AND status NOT IN ?", id, []models.ConsolidationTaskStatus{
			models.ConsolidationTaskStatusInProgress,
			models.ConsolidationTaskStatusConfirmed,
			models.ConsolidationTaskStatusCancelled,
		}).
		Updates(map[string]any{
			"status":          models.ConsolidationTaskStatusPending,
			"error_message":   &errorMsg,
			"next_attempt_at": nextAttemptAt,
			"retry_count":     retryCount,
			"tx_hash":         nil,
			"signed_tx":       nil,
			"started_at":      nil,
			"claimed_by":      nil,
			"claimed_until":   nil,
			"updated_at":      now,
			"version":         gorm.Expr("version + 1"),
		})
	if tx.Error != nil {
		return false, tx.Error
	}
	return tx.RowsAffected > 0, nil
}

func normalizeFromAddress(chain string, addr string) string {
	chain = strings.ToUpper(strings.TrimSpace(chain))
	addr = strings.TrimSpace(addr)
	switch chain {
	case "ETH", "BSC":
		return strings.ToLower(addr)
	default:
		return addr
	}
}

func normalizeTokenContract(chain string, contract string) string {
	chain = strings.ToUpper(strings.TrimSpace(chain))
	contract = strings.TrimSpace(contract)
	switch chain {
	case "ETH", "BSC":
		return strings.ToLower(contract)
	default:
		return contract
	}
}
