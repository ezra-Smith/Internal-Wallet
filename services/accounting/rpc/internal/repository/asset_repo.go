package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"internalwallet/services/accounting/rpc/internal/model"

	"gorm.io/gorm"
)

type AssetRepository interface {
	ListEnabledAssets(ctx context.Context) ([]model.AssetModel, error)
	FindByCode(ctx context.Context, code string) (*model.AssetModel, error)
	ListByCodes(ctx context.Context, codes []string) ([]model.AssetModel, error)

	CountOverview(ctx context.Context) (total, enabled, disabled int64, err error)
	List(ctx context.Context, page, pageSize int32, keyword string, status int32) ([]model.AssetModel, int64, error)

	Create(ctx context.Context, m *model.AssetModel) error
	UpdateByCode(ctx context.Context, code string, name string, precision int32) error
	UpdateStatusByCode(ctx context.Context, code string, status int32) error
	UpdateIconURLByCode(ctx context.Context, code string, iconURL string) error
}

type assetRepo struct {
	db *gorm.DB
}

func NewAssetRepository(db *gorm.DB) AssetRepository { return &assetRepo{db: db} }

func (r *assetRepo) ListEnabledAssets(ctx context.Context) ([]model.AssetModel, error) {
	var rows []model.AssetModel
	err := r.db.WithContext(ctx).Model(&model.AssetModel{}).Where("status = 1").Order("id ASC").Find(&rows).Error
	return rows, err
}

func (r *assetRepo) FindByCode(ctx context.Context, code string) (*model.AssetModel, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var m model.AssetModel
	err := r.db.WithContext(ctx).Model(&model.AssetModel{}).Where("code = ?", code).First(&m).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *assetRepo) ListByCodes(ctx context.Context, codes []string) ([]model.AssetModel, error) {
	if len(codes) == 0 {
		return nil, nil
	}
	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(codes))
	for _, c := range codes {
		c = strings.ToUpper(strings.TrimSpace(c))
		if c == "" {
			continue
		}
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		normalized = append(normalized, c)
	}
	if len(normalized) == 0 {
		return nil, nil
	}
	var rows []model.AssetModel
	err := r.db.WithContext(ctx).
		Model(&model.AssetModel{}).
		Where("code IN ?", normalized).
		Order("id ASC").
		Find(&rows).Error
	return rows, err
}

func (r *assetRepo) CountOverview(ctx context.Context) (total, enabled, disabled int64, err error) {
	q := r.db.WithContext(ctx).Model(&model.AssetModel{})
	if err := q.Count(&total).Error; err != nil {
		return 0, 0, 0, err
	}
	if err := q.Where("status = 1").Count(&enabled).Error; err != nil {
		return 0, 0, 0, err
	}
	if err := q.Where("status = 2").Count(&disabled).Error; err != nil {
		return 0, 0, 0, err
	}
	return total, enabled, disabled, nil
}

func (r *assetRepo) List(ctx context.Context, page, pageSize int32, keyword string, status int32) ([]model.AssetModel, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	q := r.db.WithContext(ctx).Model(&model.AssetModel{})
	if status == 1 || status == 2 {
		q = q.Where("status = ?", status)
	}
	kw := strings.TrimSpace(keyword)
	if kw != "" {
		kw = "%" + strings.ToUpper(kw) + "%"
		q = q.Where("(UPPER(code) LIKE ? OR UPPER(name) LIKE ?)", kw, kw)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := int((page - 1) * pageSize)
	var rows []model.AssetModel
	if err := q.Order("code ASC").Offset(offset).Limit(int(pageSize)).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *assetRepo) Create(ctx context.Context, m *model.AssetModel) error {
	if m == nil || strings.TrimSpace(m.Code) == "" {
		return fmt.Errorf("invalid asset")
	}
	m.Code = strings.ToUpper(strings.TrimSpace(m.Code))
	m.Name = strings.TrimSpace(m.Name)
	m.IconUrl = strings.TrimSpace(m.IconUrl)
	if m.Name == "" {
		m.Name = m.Code
	}
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now().Local()
	}
	m.UpdatedAt = time.Now().Local()
	m.DeletedAt = gorm.DeletedAt{}
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *assetRepo) UpdateByCode(ctx context.Context, code string, name string, precision int32) error {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return fmt.Errorf("asset not found")
	}

	updateData := map[string]interface{}{
		"updated_at": time.Now().Local(),
	}

	// 更新 name（如果提供）
	name = strings.TrimSpace(name)
	if name != "" {
		if len(name) > 64 {
			return fmt.Errorf("invalid name: too long")
		}
		updateData["name"] = name
	}

	// 更新 precision（如果提供且 > 0）
	if precision > 0 {
		if precision > 30 {
			return fmt.Errorf("invalid precision: must be between 0 and 30")
		}
		updateData["precision"] = precision
	}

	// 如果除了 updated_at 没有其他字段需要更新，则跳过
	if len(updateData) == 1 {
		return nil
	}

	res := r.db.WithContext(ctx).
		Model(&model.AssetModel{}).
		Where("code = ?", code).
		Updates(updateData)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("asset not found")
	}
	return nil
}

func (r *assetRepo) UpdateStatusByCode(ctx context.Context, code string, status int32) error {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return fmt.Errorf("asset not found")
	}
	if status != 1 && status != 2 {
		return fmt.Errorf("invalid status")
	}
	res := r.db.WithContext(ctx).
		Model(&model.AssetModel{}).
		Where("code = ?", code).
		Updates(map[string]interface{}{
			"status":     status,
			"updated_at": time.Now().Local(),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("asset not found")
	}
	return nil
}

func (r *assetRepo) UpdateIconURLByCode(ctx context.Context, code string, iconURL string) error {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return fmt.Errorf("asset not found")
	}
	res := r.db.WithContext(ctx).
		Model(&model.AssetModel{}).
		Where("code = ?", code).
		Updates(map[string]interface{}{
			"icon_url":   iconURL,
			"updated_at": time.Now().Local(),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("asset not found")
	}
	return nil
}
