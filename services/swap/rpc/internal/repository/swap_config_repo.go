package repository

import (
	"context"
	"fmt"
	"time"

	"internalwallet/services/swap/rpc/internal/model"

	"gorm.io/gorm"
)

// SwapConfigRepository 简化的Swap配置仓储接口
type SwapConfigRepository interface {
	// 基础 CRUD
	Create(ctx context.Context, m *model.SwapConfigModel) error
	GetByID(ctx context.Context, id int64) (*model.SwapConfigModel, error)
	UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error
	SoftDelete(ctx context.Context, id int64) error

	// 查询方法
	List(ctx context.Context, filters map[string]interface{}, page, pageSize int32) ([]*model.SwapConfigModel, int64, error)
	GetByProviderChainToken(ctx context.Context, provider string, chainID int64, contractAddress string) (*model.SwapConfigModel, error)
	ListByChain(ctx context.Context, chainID int64) ([]*model.SwapConfigModel, error)
	ListAllEnabled(ctx context.Context) ([]*model.SwapConfigModel, error)
}

type swapConfigRepoImpl struct {
	db *gorm.DB
}

// NewSwapConfigRepository 创建Swap配置仓储实例
func NewSwapConfigRepository(db *gorm.DB) SwapConfigRepository {
	return &swapConfigRepoImpl{db: db}
}

func (r *swapConfigRepoImpl) Create(ctx context.Context, m *model.SwapConfigModel) error {
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *swapConfigRepoImpl) GetByID(ctx context.Context, id int64) (*model.SwapConfigModel, error) {
	var m model.SwapConfigModel
	err := r.db.WithContext(ctx).
		Where("id = ?", id).
		First(&m).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *swapConfigRepoImpl) UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error {
	fields["updated_at"] = time.Now().Local()
	return r.db.WithContext(ctx).
		Model(&model.SwapConfigModel{}).
		Where("id = ?", id).
		Updates(fields).Error
}

func (r *swapConfigRepoImpl) SoftDelete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).
		Where("id = ?", id).
		Delete(&model.SwapConfigModel{}).Error
}

func (r *swapConfigRepoImpl) List(ctx context.Context, filters map[string]interface{}, page, pageSize int32) ([]*model.SwapConfigModel, int64, error) {
	var items []*model.SwapConfigModel
	var total int64

	query := r.db.WithContext(ctx).Model(&model.SwapConfigModel{})

	// 应用过滤器
	if provider, ok := filters["provider"]; ok {
		query = query.Where("provider = ?", provider)
	}
	if chainID, ok := filters["chain_id"]; ok {
		query = query.Where("chain_id = ?", chainID)
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
		Order("priority ASC, provider ASC, chain_id ASC, token_symbol ASC").
		Limit(int(pageSize)).
		Offset(int(offset)).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r *swapConfigRepoImpl) GetByProviderChainToken(ctx context.Context, provider string, chainID int64, contractAddress string) (*model.SwapConfigModel, error) {
	var m model.SwapConfigModel
	err := r.db.WithContext(ctx).
		Where("provider = ? AND chain_id = ? AND contract_address = ?", provider, chainID, contractAddress).
		First(&m).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *swapConfigRepoImpl) ListByChain(ctx context.Context, chainID int64) ([]*model.SwapConfigModel, error) {
	var items []*model.SwapConfigModel
	err := r.db.WithContext(ctx).
		Where("chain_id = ? AND is_enabled = 1", chainID).
		Order("priority ASC, token_symbol ASC").
		Find(&items).Error
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	return items, nil
}

func (r *swapConfigRepoImpl) ListAllEnabled(ctx context.Context) ([]*model.SwapConfigModel, error) {
	var items []*model.SwapConfigModel
	err := r.db.WithContext(ctx).
		Where("is_enabled = 1").
		Order("chain_id ASC, priority ASC, token_symbol ASC").
		Find(&items).Error
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	return items, nil
}
