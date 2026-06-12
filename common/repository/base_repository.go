package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

// BaseRepository 基础 Repository 接口
// 定义所有 Repository 的通用方法
type BaseRepository[T any] interface {
	// 基础 CRUD
	Create(ctx context.Context, entity *T) error
	CreateMultiple(ctx context.Context, entities []*T) error
	FindByID(ctx context.Context, id int64) (*T, error)
	Update(ctx context.Context, entity *T) error
	UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error
	Delete(ctx context.Context, id int64) error
	SoftDelete(ctx context.Context, id int64) error

	// 查询
	FindAll(ctx context.Context) ([]*T, error)
	FindByCondition(ctx context.Context, condition map[string]interface{}) ([]*T, error)
	Count(ctx context.Context, condition map[string]interface{}) (int64, error)
	Exists(ctx context.Context, id int64) (bool, error)

	// 分页
	FindWithPagination(ctx context.Context, page, pageSize int, condition map[string]interface{}) ([]*T, int64, error)

	// 事务支持
	WithTx(tx *gorm.DB) BaseRepository[T]
	GetDB() *gorm.DB
}

// baseRepositoryImpl 基础 Repository 实现
type baseRepositoryImpl[T any] struct {
	db *gorm.DB
}

// NewBaseRepository 创建基础 Repository
func NewBaseRepository[T any](db *gorm.DB) BaseRepository[T] {
	return &baseRepositoryImpl[T]{db: db}
}

// Create 创建单个实体
func (r *baseRepositoryImpl[T]) Create(ctx context.Context, entity *T) error {
	return r.db.WithContext(ctx).Create(entity).Error
}

// CreateMultiple 批量创建
func (r *baseRepositoryImpl[T]) CreateMultiple(ctx context.Context, entities []*T) error {
	if len(entities) == 0 {
		return nil
	}
	// 批量插入，每批100条
	return r.db.WithContext(ctx).CreateInBatches(entities, 100).Error
}

// FindByID 根据ID查询
func (r *baseRepositoryImpl[T]) FindByID(ctx context.Context, id int64) (*T, error) {
	var entity T
	err := r.db.WithContext(ctx).
		First(&entity, id).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("record not found")
	}
	return &entity, err
}

// Update 更新实体（简单版本，不包含乐观锁）
func (r *baseRepositoryImpl[T]) Update(ctx context.Context, entity *T) error {
	return r.db.WithContext(ctx).Save(entity).Error
}

// UpdateFields 更新指定字段
func (r *baseRepositoryImpl[T]) UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error {
	// 自动添加更新时间（DB 的 ON UPDATE 也会兜底，但这里保持显式）
	fields["updated_at"] = gorm.Expr("NOW()")

	result := r.db.WithContext(ctx).
		Model(new(T)).
		Where("id = ?", id).
		Updates(fields)

	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("update failed: record not found")
	}
	return nil
}

// Delete 物理删除
func (r *baseRepositoryImpl[T]) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Unscoped().Delete(new(T), id).Error
}

// SoftDelete 软删除
func (r *baseRepositoryImpl[T]) SoftDelete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(new(T), id).Error
}

// FindAll 查询所有（未删除的）
func (r *baseRepositoryImpl[T]) FindAll(ctx context.Context) ([]*T, error) {
	var entities []*T
	err := r.db.WithContext(ctx).Find(&entities).Error
	return entities, err
}

// FindByCondition 根据条件查询
func (r *baseRepositoryImpl[T]) FindByCondition(ctx context.Context, condition map[string]interface{}) ([]*T, error) {
	var entities []*T
	query := r.db.WithContext(ctx)

	for key, value := range condition {
		query = query.Where(key, value)
	}

	err := query.Find(&entities).Error
	return entities, err
}

// Count 统计数量
func (r *baseRepositoryImpl[T]) Count(ctx context.Context, condition map[string]interface{}) (int64, error) {
	var count int64
	query := r.db.WithContext(ctx).Model(new(T))

	for key, value := range condition {
		query = query.Where(key, value)
	}

	err := query.Count(&count).Error
	return count, err
}

// Exists 检查是否存在
func (r *baseRepositoryImpl[T]) Exists(ctx context.Context, id int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(new(T)).
		Where("id = ?", id).
		Count(&count).Error
	return count > 0, err
}

// FindWithPagination 分页查询
func (r *baseRepositoryImpl[T]) FindWithPagination(ctx context.Context, page, pageSize int, condition map[string]interface{}) ([]*T, int64, error) {
	var entities []*T
	var total int64

	// 构建查询
	query := r.db.WithContext(ctx).Model(new(T))
	for key, value := range condition {
		query = query.Where(key, value)
	}

	// 计算总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	err := query.Offset(offset).Limit(pageSize).Find(&entities).Error

	return entities, total, err
}

// WithTx 返回带事务的 Repository
func (r *baseRepositoryImpl[T]) WithTx(tx *gorm.DB) BaseRepository[T] {
	return &baseRepositoryImpl[T]{db: tx}
}

// GetDB 获取数据库连接
func (r *baseRepositoryImpl[T]) GetDB() *gorm.DB {
	return r.db
}
