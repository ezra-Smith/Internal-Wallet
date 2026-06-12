package repository

import (
	"context"
	"time"

	"internalwallet/services/swap/rpc/internal/model"

	"gorm.io/gorm"
)

// SwapProviderRepository Swap服务商仓储接口
type SwapProviderRepository interface {
	// 基础 CRUD
	Create(ctx context.Context, m *model.SwapProviderModel) error
	GetByID(ctx context.Context, id int64) (*model.SwapProviderModel, error)
	GetByCode(ctx context.Context, code string) (*model.SwapProviderModel, error)
	UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error
	SoftDelete(ctx context.Context, id int64) error

	// 查询方法
	List(ctx context.Context, filters map[string]interface{}, page, pageSize int32) ([]*model.SwapProviderModel, int64, error)
	ListEnabled(ctx context.Context) ([]*model.SwapProviderModel, error)
	Enable(ctx context.Context, id int64) error
	Disable(ctx context.Context, id int64) error
}

type swapProviderRepoImpl struct {
	db *gorm.DB
}

// NewSwapProviderRepository 创建Swap服务商仓储实例
func NewSwapProviderRepository(db *gorm.DB) SwapProviderRepository {
	return &swapProviderRepoImpl{db: db}
}

func (r *swapProviderRepoImpl) Create(ctx context.Context, m *model.SwapProviderModel) error {
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *swapProviderRepoImpl) GetByID(ctx context.Context, id int64) (*model.SwapProviderModel, error) {
	var m model.SwapProviderModel
	err := r.db.WithContext(ctx).
		Where("id = ?", id).
		First(&m).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *swapProviderRepoImpl) GetByCode(ctx context.Context, code string) (*model.SwapProviderModel, error) {
	var m model.SwapProviderModel
	err := r.db.WithContext(ctx).
		Where("provider_code = ?", code).
		First(&m).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *swapProviderRepoImpl) UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error {
	fields["updated_at"] = time.Now().Local()
	return r.db.WithContext(ctx).
		Model(&model.SwapProviderModel{}).
		Where("id = ?", id).
		Updates(fields).Error
}

func (r *swapProviderRepoImpl) SoftDelete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).
		Where("id = ?", id).
		Delete(&model.SwapProviderModel{}).Error
}

func (r *swapProviderRepoImpl) List(ctx context.Context, filters map[string]interface{}, page, pageSize int32) ([]*model.SwapProviderModel, int64, error) {
	var items []*model.SwapProviderModel
	var total int64

	query := r.db.WithContext(ctx).Model(&model.SwapProviderModel{})

	// 应用过滤器
	if providerCode, ok := filters["provider_code"]; ok {
		query = query.Where("provider_code = ?", providerCode)
	}
	if isEnabled, ok := filters["is_enabled"]; ok {
		query = query.Where("is_enabled = ?", isEnabled)
	}

	// 计算总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	if err := query.
		Order("id ASC").
		Limit(int(pageSize)).
		Offset(int(offset)).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r *swapProviderRepoImpl) Enable(ctx context.Context, id int64) error {
	return r.UpdateFields(ctx, id, map[string]interface{}{
		"is_enabled": true,
	})
}

func (r *swapProviderRepoImpl) Disable(ctx context.Context, id int64) error {
	return r.UpdateFields(ctx, id, map[string]interface{}{
		"is_enabled": false,
	})
}

// ListEnabled 查询所有启用的服务商（用于下拉框）
func (r *swapProviderRepoImpl) ListEnabled(ctx context.Context) ([]*model.SwapProviderModel, error) {
	var items []*model.SwapProviderModel
	err := r.db.WithContext(ctx).
		Where("is_enabled = ?", true).
		Where("deleted_at IS NULL").
		Order("id ASC").
		Find(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}
