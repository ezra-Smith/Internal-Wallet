package repository

import (
	"context"
	"time"

	"internalwallet/services/swap/rpc/internal/model"

	"gorm.io/gorm"
)

// SwapProjectEnabledRepository 项目启用配置仓储接口
type SwapProjectEnabledRepository interface {
	// 基础 CRUD
	Create(ctx context.Context, m *model.SwapProjectEnabledModel) error
	GetByID(ctx context.Context, id int64) (*model.SwapProjectEnabledModel, error)
	UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error
	SoftDelete(ctx context.Context, id int64) error

	// 查询方法
	ListByProject(ctx context.Context, projectName string, page, pageSize int32) ([]*model.SwapProjectEnabledModel, int64, error)
	GetByProjectAndConfig(ctx context.Context, projectName string, configID int64) (*model.SwapProjectEnabledModel, error)

	// 联表查询（返回完整配置信息）
	ListProjectConfigsWithDetails(ctx context.Context, projectName string, page, pageSize int32) ([]map[string]interface{}, int64, error)
}

type swapProjectEnabledRepoImpl struct {
	db *gorm.DB
}

// NewSwapProjectEnabledRepository 创建项目启用配置仓储实例
func NewSwapProjectEnabledRepository(db *gorm.DB) SwapProjectEnabledRepository {
	return &swapProjectEnabledRepoImpl{db: db}
}

func (r *swapProjectEnabledRepoImpl) Create(ctx context.Context, m *model.SwapProjectEnabledModel) error {
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *swapProjectEnabledRepoImpl) GetByID(ctx context.Context, id int64) (*model.SwapProjectEnabledModel, error) {
	var m model.SwapProjectEnabledModel
	err := r.db.WithContext(ctx).
		Where("id = ?", id).
		First(&m).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *swapProjectEnabledRepoImpl) UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error {
	fields["updated_at"] = time.Now().Local()
	return r.db.WithContext(ctx).
		Model(&model.SwapProjectEnabledModel{}).
		Where("id = ?", id).
		Updates(fields).Error
}

func (r *swapProjectEnabledRepoImpl) SoftDelete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).
		Where("id = ?", id).
		Delete(&model.SwapProjectEnabledModel{}).Error
}

func (r *swapProjectEnabledRepoImpl) ListByProject(ctx context.Context, projectName string, page, pageSize int32) ([]*model.SwapProjectEnabledModel, int64, error) {
	var items []*model.SwapProjectEnabledModel
	var total int64

	query := r.db.WithContext(ctx).
		Model(&model.SwapProjectEnabledModel{}).
		Where("project_name = ?", projectName)

	// 计算总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	if err := query.
		Order("id DESC").
		Limit(int(pageSize)).
		Offset(int(offset)).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r *swapProjectEnabledRepoImpl) GetByProjectAndConfig(ctx context.Context, projectName string, configID int64) (*model.SwapProjectEnabledModel, error) {
	var m model.SwapProjectEnabledModel
	err := r.db.WithContext(ctx).
		Where("project_name = ? AND config_id = ?", projectName, configID).
		First(&m).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *swapProjectEnabledRepoImpl) ListProjectConfigsWithDetails(ctx context.Context, projectName string, page, pageSize int32) ([]map[string]interface{}, int64, error) {
	var results []map[string]interface{}
	var total int64

	// 联表查询：swap_project_enabled JOIN swap_configs
	query := r.db.WithContext(ctx).
		Table("swap_project_enabled pe").
		Select(`
			pe.id as enabled_id,
			pe.project_name,
			pe.is_enabled as project_enabled,
			c.id as config_id,
			c.provider_id,
			c.provider,
			c.provider_name,
			c.chain_id,
			c.chain_name,
			c.chain_symbol,
			c.router_address,
			c.token_symbol,
			c.token_name,
			c.contract_address,
			c.decimals,
			c.icon_url,
			c.priority,
			c.min_swap_amount_usd,
			c.max_swap_amount_usd,
			c.default_slippage,
			c.max_slippage,
			c.fee_rate,
			c.is_enabled as global_enabled,
			pe.created_at,
			pe.updated_at
		`).
		Joins("INNER JOIN swap_configs c ON pe.config_id = c.id").
		Where("pe.project_name = ? AND pe.deleted_at IS NULL AND c.deleted_at IS NULL", projectName)

	// 计算总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	if err := query.
		Order("c.priority ASC, c.chain_id ASC, c.token_symbol ASC").
		Limit(int(pageSize)).
		Offset(int(offset)).
		Find(&results).Error; err != nil {
		return nil, 0, err
	}

	return results, total, nil
}
