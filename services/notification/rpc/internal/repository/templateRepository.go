package repository

import (
	"context"
	commonRepo "internalwallet/common/repository"
	"internalwallet/services/notification/rpc/internal/model"

	"gorm.io/gorm"
)

// TemplateRepository 模板仓储接口
// 嵌入 BaseRepository，继承基础 CRUD 方法
type TemplateRepository interface {
	commonRepo.BaseRepository[model.NotificationTemplate]

	// 业务特定方法
	FindByCode(ctx context.Context, templateCode, language string) (*model.NotificationTemplate, error)
	FindByType(ctx context.Context, notificationType string, language string) ([]*model.NotificationTemplate, error)
	List(ctx context.Context, notificationType, language string, page, pageSize int) ([]*model.NotificationTemplate, int64, error)
}

type templateRepository struct {
	commonRepo.BaseRepository[model.NotificationTemplate]
}

// NewTemplateRepository 创建模板仓储实例
func NewTemplateRepository(db *gorm.DB) TemplateRepository {
	return &templateRepository{
		BaseRepository: commonRepo.NewBaseRepository[model.NotificationTemplate](db),
	}
}

// FindByCode 根据模板代码和语言查询模板
func (r *templateRepository) FindByCode(ctx context.Context, templateCode, language string) (*model.NotificationTemplate, error) {
	var template model.NotificationTemplate
	err := r.GetDB().WithContext(ctx).
		Where("template_code = ? AND language = ?", templateCode, language).
		Where("is_enabled = ?", true).
		First(&template).Error
	if err != nil {
		return nil, err
	}
	return &template, nil
}

// FindByType 根据通知类型和语言查询模板列表
func (r *templateRepository) FindByType(ctx context.Context, notificationType string, language string) ([]*model.NotificationTemplate, error) {
	var templates []*model.NotificationTemplate
	err := r.GetDB().WithContext(ctx).
		Where("notification_type = ? AND language = ?", notificationType, language).
		Where("is_enabled = ?", true).
		Order("priority DESC, created_at DESC").
		Find(&templates).Error
	return templates, err
}

// List 查询模板列表（分页）
func (r *templateRepository) List(ctx context.Context, notificationType, language string, page, pageSize int) ([]*model.NotificationTemplate, int64, error) {
	var templates []*model.NotificationTemplate
	var total int64

	query := r.GetDB().WithContext(ctx).Model(&model.NotificationTemplate{})

	// 如果指定了通知类型，添加过滤条件
	if notificationType != "" && notificationType != "unspecified" {
		query = query.Where("notification_type = ?", notificationType)
	}

	// 如果指定了语言，添加过滤条件
	if language != "" {
		query = query.Where("language = ?", language)
	}

	// 计算总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	err := query.Order("notification_type, language, priority DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&templates).Error

	return templates, total, err
}
