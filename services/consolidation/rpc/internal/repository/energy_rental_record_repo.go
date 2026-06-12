package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"internalwallet/services/consolidation/rpc/internal/models"

	"gorm.io/gorm"
)

type EnergyRentalRecordRepository interface {
	CreateIgnoreDuplicate(ctx context.Context, rec *models.EnergyRentalRecord) (created bool, err error)
	GetByID(ctx context.Context, id int64) (*models.EnergyRentalRecord, error)
	GetByOrderID(ctx context.Context, orderID string) (*models.EnergyRentalRecord, error)
	List(ctx context.Context, req ListEnergyRentalRecordsRequest) (records []models.EnergyRentalRecord, total int64, err error)
	UpdateStatus(ctx context.Context, id int64, status models.EnergyRentalStatus, providerResponse []byte, errMsg *string, confirmedAt *time.Time) error
}

type energyRentalRecordRepo struct {
	db *gorm.DB
}

func NewEnergyRentalRecordRepository(db *gorm.DB) EnergyRentalRecordRepository {
	return &energyRentalRecordRepo{db: db}
}

func (r *energyRentalRecordRepo) CreateIgnoreDuplicate(ctx context.Context, rec *models.EnergyRentalRecord) (bool, error) {
	if rec == nil {
		return false, fmt.Errorf("record is nil")
	}
	if r.db == nil {
		return false, fmt.Errorf("db not configured")
	}
	err := r.db.WithContext(ctx).Create(rec).Error
	if err != nil {
		if isDuplicateKeyError(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *energyRentalRecordRepo) GetByID(ctx context.Context, id int64) (*models.EnergyRentalRecord, error) {
	if r.db == nil {
		return nil, fmt.Errorf("db not configured")
	}
	var rec models.EnergyRentalRecord
	if err := r.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&rec).Error; err != nil {
		return nil, err
	}
	return &rec, nil
}

func (r *energyRentalRecordRepo) GetByOrderID(ctx context.Context, orderID string) (*models.EnergyRentalRecord, error) {
	if r.db == nil {
		return nil, fmt.Errorf("db not configured")
	}
	var rec models.EnergyRentalRecord
	if err := r.db.WithContext(ctx).Where("order_id = ? AND deleted_at IS NULL", orderID).First(&rec).Error; err != nil {
		return nil, err
	}
	return &rec, nil
}

type ListEnergyRentalRecordsRequest struct {
	Page          int32
	PageSize      int32
	Receiver      *string
	Provider      *string
	Status        *models.EnergyRentalStatus
	OrderID       *string
	CreatedAtFrom *time.Time
	CreatedAtTo   *time.Time
}

func (r *energyRentalRecordRepo) List(ctx context.Context, req ListEnergyRentalRecordsRequest) ([]models.EnergyRentalRecord, int64, error) {
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

	q := r.db.WithContext(ctx).Model(&models.EnergyRentalRecord{}).Where("deleted_at IS NULL")
	if req.Receiver != nil {
		addr := strings.TrimSpace(*req.Receiver)
		if addr != "" {
			q = q.Where("receiver_address = ?", addr)
		}
	}
	if req.Provider != nil {
		provider := strings.TrimSpace(*req.Provider)
		if provider != "" {
			q = q.Where("provider = ?", provider)
		}
	}
	if req.Status != nil && *req.Status != 0 {
		q = q.Where("status = ?", *req.Status)
	}
	if req.OrderID != nil {
		orderID := strings.TrimSpace(*req.OrderID)
		if orderID != "" {
			q = q.Where("order_id = ?", orderID)
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

	var items []models.EnergyRentalRecord
	offset := int((page - 1) * pageSize)
	if err := q.Order("created_at DESC").Offset(offset).Limit(int(pageSize)).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *energyRentalRecordRepo) UpdateStatus(ctx context.Context, id int64, status models.EnergyRentalStatus, providerResponse []byte, errMsg *string, confirmedAt *time.Time) error {
	if r.db == nil {
		return fmt.Errorf("db not configured")
	}
	now := time.Now().Local()
	updates := map[string]any{
		"status":        status,
		"updated_at":    now,
		"error_message": errMsg,
		"confirmed_at":  confirmedAt,
	}
	if providerResponse != nil {
		updates["provider_response"] = providerResponse
	}
	return r.db.WithContext(ctx).Model(&models.EnergyRentalRecord{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(updates).Error
}
