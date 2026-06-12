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

type VaultAdjustmentListFilter struct {
	Statuses       []string
	Network        string
	ChainID        int64
	Currency       string
	AdjustmentType string
	DateFrom       *time.Time
	DateTo         *time.Time
	SortBy         string
	SortOrder      string
}

type VaultAdjustmentRepository interface {
	WithTx(tx *gorm.DB) VaultAdjustmentRepository
	GetDB() *gorm.DB

	Create(ctx context.Context, m *model.VaultAdjustmentModel) error
	FindByID(ctx context.Context, id int64) (*model.VaultAdjustmentModel, error)
	FindByIDForUpdate(ctx context.Context, id int64) (*model.VaultAdjustmentModel, error)
	List(ctx context.Context, page, pageSize int32, f VaultAdjustmentListFilter) ([]*model.VaultAdjustmentModel, int64, error)
	UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error

	ExistsPendingByNetworkAndCurrency(ctx context.Context, networkID int64, currency string) (bool, error)
	CountByStatuses(ctx context.Context, statuses []string) (int64, error)
}

type vaultAdjustmentRepo struct {
	db *gorm.DB
}

func NewVaultAdjustmentRepository(db *gorm.DB) VaultAdjustmentRepository {
	return &vaultAdjustmentRepo{db: db}
}
func (r *vaultAdjustmentRepo) WithTx(tx *gorm.DB) VaultAdjustmentRepository {
	return &vaultAdjustmentRepo{db: tx}
}
func (r *vaultAdjustmentRepo) GetDB() *gorm.DB { return r.db }

func (r *vaultAdjustmentRepo) Create(ctx context.Context, m *model.VaultAdjustmentModel) error {
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *vaultAdjustmentRepo) FindByID(ctx context.Context, id int64) (*model.VaultAdjustmentModel, error) {
	var m model.VaultAdjustmentModel
	err := r.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", id).
		First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("adjustment not found")
	}
	return &m, err
}

func (r *vaultAdjustmentRepo) FindByIDForUpdate(ctx context.Context, id int64) (*model.VaultAdjustmentModel, error) {
	var m model.VaultAdjustmentModel
	err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND deleted_at IS NULL", id).
		First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("adjustment not found")
	}
	return &m, err
}

func (r *vaultAdjustmentRepo) ExistsPendingByNetworkAndCurrency(ctx context.Context, networkID int64, currency string) (bool, error) {
	currency = strings.TrimSpace(currency)
	if networkID <= 0 || currency == "" {
		return false, nil
	}
	var count int64
	err := r.db.WithContext(ctx).Model(&model.VaultAdjustmentModel{}).
		Where("network_id = ? AND currency = ? AND status = ? AND deleted_at IS NULL", networkID, currency, "pending_approval").
		Count(&count).Error
	return count > 0, err
}

func (r *vaultAdjustmentRepo) CountByStatuses(ctx context.Context, statuses []string) (int64, error) {
	q := r.db.WithContext(ctx).Model(&model.VaultAdjustmentModel{}).Where("deleted_at IS NULL")
	if len(statuses) > 0 {
		q = q.Where("status IN ?", statuses)
	}
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *vaultAdjustmentRepo) List(ctx context.Context, page, pageSize int32, f VaultAdjustmentListFilter) ([]*model.VaultAdjustmentModel, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	q := r.db.WithContext(ctx).Model(&model.VaultAdjustmentModel{}).Where("deleted_at IS NULL")
	if len(f.Statuses) > 0 {
		q = q.Where("status IN ?", f.Statuses)
	}
	if strings.TrimSpace(f.Network) != "" {
		q = q.Where("network = ?", strings.TrimSpace(f.Network))
	}
	if f.ChainID != 0 {
		q = q.Where("chain_id = ?", f.ChainID)
	}
	if strings.TrimSpace(f.Currency) != "" {
		q = q.Where("currency = ?", strings.TrimSpace(f.Currency))
	}
	if strings.TrimSpace(f.AdjustmentType) != "" {
		q = q.Where("adjustment_type = ?", strings.TrimSpace(f.AdjustmentType))
	}
	if f.DateFrom != nil && !f.DateFrom.IsZero() {
		q = q.Where("created_at >= ?", *f.DateFrom)
	}
	if f.DateTo != nil && !f.DateTo.IsZero() {
		q = q.Where("created_at <= ?", *f.DateTo)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	orderCol := "created_at"
	switch strings.TrimSpace(f.SortBy) {
	case "", "created_at":
		orderCol = "created_at"
	case "amount":
		orderCol = "amount_raw"
	case "status":
		orderCol = "status"
	}
	orderDir := "DESC"
	if strings.EqualFold(strings.TrimSpace(f.SortOrder), "asc") {
		orderDir = "ASC"
	}

	var items []*model.VaultAdjustmentModel
	offset := int((page - 1) * pageSize)
	err := q.Order(orderCol + " " + orderDir).Offset(offset).Limit(int(pageSize)).Find(&items).Error
	return items, total, err
}

func (r *vaultAdjustmentRepo) UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error {
	res := r.db.WithContext(ctx).
		Model(&model.VaultAdjustmentModel{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(fields)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("adjustment not found")
	}
	return nil
}
