package repository

import (
	"context"
	"errors"
	"gorm.io/gorm"
	"internalwallet/services/chainsync/rpc/internal/model"
)

// AddressMonitorRepository 地址监控仓库接口
type AddressMonitorRepository interface {
	// 基础CRUD
	Create(ctx context.Context, monitor *model.AddressMonitorGorm) error
	GetByID(ctx context.Context, id int64) (*model.AddressMonitorGorm, error)
	GetByMonitorID(ctx context.Context, monitorID string) (*model.AddressMonitorGorm, error)
	Update(ctx context.Context, monitor *model.AddressMonitorGorm) error
	Delete(ctx context.Context, id int64) error

	// 查询方法
	GetByChain(ctx context.Context, chain string) ([]*model.AddressMonitorGorm, error)
	GetByAddress(ctx context.Context, chain, address string) ([]*model.AddressMonitorGorm, error)
	GetByChainAndType(ctx context.Context, chain, monitorType string) ([]*model.AddressMonitorGorm, error)
	GetActiveMonitors(ctx context.Context, chain string) ([]*model.AddressMonitorGorm, error)
	GetMonitorsWithPagination(ctx context.Context, chain, monitorType string, page, pageSize int) ([]*model.AddressMonitorGorm, int64, error)

	// 批量操作
	BatchUpdateActivity(ctx context.Context, monitorIDs []string, lastActivity int64) error
	DeactivateByIDs(ctx context.Context, monitorIDs []string) error
}

// addressMonitorRepository 地址监控仓库实现
type addressMonitorRepository struct {
	db *gorm.DB
}

// NewAddressMonitorRepository 创建地址监控仓库
func NewAddressMonitorRepository(db *gorm.DB) AddressMonitorRepository {
	return &addressMonitorRepository{db: db}
}

// Create 创建地址监控记录
func (r *addressMonitorRepository) Create(ctx context.Context, monitor *model.AddressMonitorGorm) error {
	return r.db.WithContext(ctx).Create(monitor).Error
}

// GetByID 根据ID获取地址监控
func (r *addressMonitorRepository) GetByID(ctx context.Context, id int64) (*model.AddressMonitorGorm, error) {
	var monitor model.AddressMonitorGorm
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&monitor).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	return &monitor, nil
}

// GetByMonitorID 根据监控ID获取地址监控
func (r *addressMonitorRepository) GetByMonitorID(ctx context.Context, monitorID string) (*model.AddressMonitorGorm, error) {
	var monitor model.AddressMonitorGorm
	err := r.db.WithContext(ctx).Where("monitor_id = ?", monitorID).First(&monitor).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	return &monitor, nil
}

// Update 更新地址监控记录
func (r *addressMonitorRepository) Update(ctx context.Context, monitor *model.AddressMonitorGorm) error {
	return r.db.WithContext(ctx).Model(monitor).Where("id = ?", monitor.ID).Updates(monitor).Error
}

// Delete 软删除地址监控记录
func (r *addressMonitorRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.AddressMonitorGorm{}).Error
}

// GetByChain 根据链获取地址监控列表
func (r *addressMonitorRepository) GetByChain(ctx context.Context, chain string) ([]*model.AddressMonitorGorm, error) {
	var monitors []*model.AddressMonitorGorm
	err := r.db.WithContext(ctx).Where("chain = ?", chain).Order("created_at DESC").Find(&monitors).Error
	return monitors, err
}

// GetByAddress 根据地址获取地址监控列表
func (r *addressMonitorRepository) GetByAddress(ctx context.Context, chain, address string) ([]*model.AddressMonitorGorm, error) {
	var monitors []*model.AddressMonitorGorm
	err := r.db.WithContext(ctx).Where("chain = ? AND address = ?", chain, address).Order("created_at DESC").Find(&monitors).Error
	return monitors, err
}

// GetByChainAndType 根据链和监控类型获取地址监控列表
func (r *addressMonitorRepository) GetByChainAndType(ctx context.Context, chain, monitorType string) ([]*model.AddressMonitorGorm, error) {
	var monitors []*model.AddressMonitorGorm
	err := r.db.WithContext(ctx).Where("chain = ? AND monitor_type = ?", chain, monitorType).
		Order("created_at DESC").Find(&monitors).Error
	return monitors, err
}

// GetActiveMonitors 获取活跃的地址监控列表
func (r *addressMonitorRepository) GetActiveMonitors(ctx context.Context, chain string) ([]*model.AddressMonitorGorm, error) {
	var monitors []*model.AddressMonitorGorm
	query := r.db.WithContext(ctx).Where("active = ?", true)
	if chain != "" {
		query = query.Where("chain = ?", chain)
	}
	err := query.Order("priority DESC, created_at ASC").Find(&monitors).Error
	return monitors, err
}

// GetMonitorsWithPagination 分页获取地址监控列表
func (r *addressMonitorRepository) GetMonitorsWithPagination(ctx context.Context, chain, monitorType string, page, pageSize int) ([]*model.AddressMonitorGorm, int64, error) {
	var monitors []*model.AddressMonitorGorm
	var total int64

	query := r.db.WithContext(ctx).Model(&model.AddressMonitorGorm{})

	// 添加过滤条件
	if chain != "" {
		query = query.Where("chain = ?", chain)
	}
	if monitorType != "" {
		query = query.Where("monitor_type = ?", monitorType)
	}

	// 获取总数
	err := query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := page * pageSize
	err = query.Order("created_at DESC").Limit(pageSize).Offset(offset).Find(&monitors).Error
	return monitors, total, err
}

// BatchUpdateActivity 批量更新最后活动时间
func (r *addressMonitorRepository) BatchUpdateActivity(ctx context.Context, monitorIDs []string, lastActivity int64) error {
	if len(monitorIDs) == 0 {
		return nil
	}

	return r.db.WithContext(ctx).Model(&model.AddressMonitorGorm{}).
		Where("monitor_id IN ?", monitorIDs).
		Update("last_activity", lastActivity).Error
}

// DeactivateByIDs 根据ID批量停用监控
func (r *addressMonitorRepository) DeactivateByIDs(ctx context.Context, monitorIDs []string) error {
	if len(monitorIDs) == 0 {
		return nil
	}

	return r.db.WithContext(ctx).Model(&model.AddressMonitorGorm{}).
		Where("monitor_id IN ?", monitorIDs).
		Update("active", false).Error
}
