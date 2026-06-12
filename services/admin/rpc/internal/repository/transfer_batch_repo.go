package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"internalwallet/services/admin/rpc/internal/model"

	"gorm.io/gorm"
)

type TransferBatchListFilter struct {
	Status      string
	Network     string
	Currency    string
	Keyword     string
	CreatedFrom *time.Time
	CreatedTo   *time.Time
}

type TransferBatchRepository interface {
	WithTx(tx *gorm.DB) TransferBatchRepository
	GetDB() *gorm.DB

	Create(ctx context.Context, m *model.TransferBatchModel) error
	FindByID(ctx context.Context, batchID string) (*model.TransferBatchModel, error)
	List(ctx context.Context, page, pageSize int32, f TransferBatchListFilter) ([]*model.TransferBatchModel, int64, error)
	UpdateFields(ctx context.Context, batchID string, fields map[string]interface{}) error
}

type transferBatchRepo struct {
	db *gorm.DB
}

func NewTransferBatchRepository(db *gorm.DB) TransferBatchRepository {
	return &transferBatchRepo{db: db}
}
func (r *transferBatchRepo) WithTx(tx *gorm.DB) TransferBatchRepository {
	return &transferBatchRepo{db: tx}
}
func (r *transferBatchRepo) GetDB() *gorm.DB { return r.db }

func (r *transferBatchRepo) Create(ctx context.Context, m *model.TransferBatchModel) error {
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *transferBatchRepo) FindByID(ctx context.Context, batchID string) (*model.TransferBatchModel, error) {
	batchID = strings.TrimSpace(batchID)
	if batchID == "" {
		return nil, fmt.Errorf("batch not found")
	}
	var m model.TransferBatchModel
	err := r.db.WithContext(ctx).
		Where("batch_id = ?", batchID).
		First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("batch not found")
	}
	return &m, err
}

func (r *transferBatchRepo) List(ctx context.Context, page, pageSize int32, f TransferBatchListFilter) ([]*model.TransferBatchModel, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}

	query := r.db.WithContext(ctx).Model(&model.TransferBatchModel{})
	if strings.TrimSpace(f.Status) != "" {
		query = query.Where("status = ?", strings.TrimSpace(f.Status))
	}
	if strings.TrimSpace(f.Network) != "" {
		query = query.Where("network = ?", strings.TrimSpace(f.Network))
	}
	if strings.TrimSpace(f.Currency) != "" {
		query = query.Where("currency = ?", strings.TrimSpace(f.Currency))
	}
	if strings.TrimSpace(f.Keyword) != "" {
		kw := "%" + strings.TrimSpace(f.Keyword) + "%"
		query = query.Where("(batch_id LIKE ? OR name LIKE ?)", kw, kw)
	}
	if f.CreatedFrom != nil && !f.CreatedFrom.IsZero() {
		query = query.Where("created_at >= ?", *f.CreatedFrom)
	}
	if f.CreatedTo != nil && !f.CreatedTo.IsZero() {
		query = query.Where("created_at <= ?", *f.CreatedTo)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := int((page - 1) * pageSize)
	var items []*model.TransferBatchModel
	err := query.Order("created_at DESC").Offset(offset).Limit(int(pageSize)).Find(&items).Error
	return items, total, err
}

func (r *transferBatchRepo) UpdateFields(ctx context.Context, batchID string, fields map[string]interface{}) error {
	batchID = strings.TrimSpace(batchID)
	if batchID == "" {
		return fmt.Errorf("batch not found")
	}
	res := r.db.WithContext(ctx).
		Model(&model.TransferBatchModel{}).
		Where("batch_id = ?", batchID).
		Updates(fields)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("batch not found")
	}
	return nil
}
