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

type BlacklistAddressFilter struct {
	Keyword       string
	RiskLevel     string
	Network       string
	MonitorStatus string
}

type BlacklistAddressStats struct {
	Total      int64
	HighRisk   int64
	Monitoring int64
	TotalHits  int64
}

type BlacklistAddressRepository interface {
	WithTx(tx *gorm.DB) BlacklistAddressRepository
	GetDB() *gorm.DB

	Create(ctx context.Context, m *model.BlacklistAddressModel) error
	FindByID(ctx context.Context, id int64) (*model.BlacklistAddressModel, error)
	List(ctx context.Context, page, pageSize int32, f BlacklistAddressFilter) ([]*model.BlacklistAddressModel, int64, error)
	Stats(ctx context.Context, f BlacklistAddressFilter) (BlacklistAddressStats, error)
	UpdateMonitorStatus(ctx context.Context, id int64, monitorStatus string, now time.Time) error
	SoftDelete(ctx context.Context, id int64, now time.Time) error
}

type blacklistAddressRepo struct {
	db *gorm.DB
}

func NewBlacklistAddressRepository(db *gorm.DB) BlacklistAddressRepository {
	return &blacklistAddressRepo{db: db}
}

func (r *blacklistAddressRepo) WithTx(tx *gorm.DB) BlacklistAddressRepository {
	return &blacklistAddressRepo{db: tx}
}

func (r *blacklistAddressRepo) GetDB() *gorm.DB { return r.db }

func applyBlacklistAddressFilters(q *gorm.DB, f BlacklistAddressFilter) *gorm.DB {
	q = q.Where("deleted_at IS NULL")

	if kw := strings.TrimSpace(f.Keyword); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("(address LIKE ? OR reason LIKE ? OR source LIKE ?)", like, like, like)
	}

	if v := strings.TrimSpace(f.RiskLevel); v != "" && v != "all" {
		q = q.Where("risk_level = ?", v)
	}
	if v := strings.TrimSpace(f.Network); v != "" && v != "all" {
		q = q.Where("network = ?", v)
	}
	if v := strings.TrimSpace(f.MonitorStatus); v != "" && v != "all" {
		q = q.Where("monitor_status = ?", v)
	}

	return q
}

func (r *blacklistAddressRepo) Create(ctx context.Context, m *model.BlacklistAddressModel) error {
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *blacklistAddressRepo) FindByID(ctx context.Context, id int64) (*model.BlacklistAddressModel, error) {
	if id <= 0 {
		return nil, fmt.Errorf("blacklist address not found")
	}
	var m model.BlacklistAddressModel
	err := r.db.WithContext(ctx).
		Model(&model.BlacklistAddressModel{}).
		Where("id = ? AND deleted_at IS NULL", id).
		First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("blacklist address not found")
	}
	return &m, err
}

func (r *blacklistAddressRepo) List(ctx context.Context, page, pageSize int32, f BlacklistAddressFilter) ([]*model.BlacklistAddressModel, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}

	query := applyBlacklistAddressFilters(r.db.WithContext(ctx).Model(&model.BlacklistAddressModel{}), f)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []*model.BlacklistAddressModel
	offset := int((page - 1) * pageSize)
	err := query.Order("created_at DESC, id DESC").Offset(offset).Limit(int(pageSize)).Find(&items).Error
	return items, total, err
}

func (r *blacklistAddressRepo) Stats(ctx context.Context, f BlacklistAddressFilter) (BlacklistAddressStats, error) {
	var out BlacklistAddressStats

	q := applyBlacklistAddressFilters(r.db.WithContext(ctx).Model(&model.BlacklistAddressModel{}), f)
	if err := q.Count(&out.Total).Error; err != nil {
		return out, err
	}

	qHigh := applyBlacklistAddressFilters(r.db.WithContext(ctx).Model(&model.BlacklistAddressModel{}), f).
		Where("risk_level = ?", "high")
	if err := qHigh.Count(&out.HighRisk).Error; err != nil {
		return out, err
	}

	qMon := applyBlacklistAddressFilters(r.db.WithContext(ctx).Model(&model.BlacklistAddressModel{}), f).
		Where("monitor_status = ?", "active")
	if err := qMon.Count(&out.Monitoring).Error; err != nil {
		return out, err
	}

	type sumRow struct {
		TotalHits int64 `gorm:"column:total_hits"`
	}
	var row sumRow
	if err := applyBlacklistAddressFilters(r.db.WithContext(ctx).Model(&model.BlacklistAddressModel{}), f).
		Select("COALESCE(SUM(hit_count), 0) AS total_hits").
		Scan(&row).Error; err != nil {
		return out, err
	}
	out.TotalHits = row.TotalHits

	return out, nil
}

func (r *blacklistAddressRepo) UpdateMonitorStatus(ctx context.Context, id int64, monitorStatus string, now time.Time) error {
	if id <= 0 {
		return fmt.Errorf("blacklist address not found")
	}
	res := r.db.WithContext(ctx).
		Model(&model.BlacklistAddressModel{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]interface{}{
			"monitor_status": strings.TrimSpace(monitorStatus),
			"updated_at":     &now,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("blacklist address not found")
	}
	return nil
}

func (r *blacklistAddressRepo) SoftDelete(ctx context.Context, id int64, now time.Time) error {
	if id <= 0 {
		return fmt.Errorf("blacklist address not found")
	}
	res := r.db.WithContext(ctx).
		Model(&model.BlacklistAddressModel{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]interface{}{
			"deleted_at": &now,
			"updated_at": &now,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("blacklist address not found")
	}
	return nil
}
