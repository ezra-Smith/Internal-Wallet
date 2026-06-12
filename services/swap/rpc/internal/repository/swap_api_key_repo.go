package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"internalwallet/services/swap/rpc/internal/model"

	"gorm.io/gorm"
)

type SwapApiKeyRepository interface {
	WithTx(tx *gorm.DB) SwapApiKeyRepository
	GetDB() *gorm.DB

	Create(ctx context.Context, m *model.SwapSvcApiKeyModel) error
	FindByID(ctx context.Context, id int64) (*model.SwapSvcApiKeyModel, error)
	FindByKeyHash(ctx context.Context, keyHash string) (*model.SwapSvcApiKeyModel, error)
	List(ctx context.Context, page, pageSize int32) ([]*model.SwapSvcApiKeyModel, int64, error)
	ListByProjectName(ctx context.Context, projectName string) ([]*model.SwapSvcApiKeyModel, error)
	UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error
}

type swapApiKeyRepo struct {
	db *gorm.DB
}

func NewSwapApiKeyRepository(db *gorm.DB) SwapApiKeyRepository {
	return &swapApiKeyRepo{db: db}
}

func (r *swapApiKeyRepo) WithTx(tx *gorm.DB) SwapApiKeyRepository { return &swapApiKeyRepo{db: tx} }
func (r *swapApiKeyRepo) GetDB() *gorm.DB                         { return r.db }

func (r *swapApiKeyRepo) Create(ctx context.Context, m *model.SwapSvcApiKeyModel) error {
	if m == nil {
		return fmt.Errorf("nil model")
	}
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *swapApiKeyRepo) FindByID(ctx context.Context, id int64) (*model.SwapSvcApiKeyModel, error) {
	if id <= 0 {
		return nil, fmt.Errorf("not found")
	}
	var m model.SwapSvcApiKeyModel
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("not found")
	}
	return &m, err
}

func (r *swapApiKeyRepo) FindByKeyHash(ctx context.Context, keyHash string) (*model.SwapSvcApiKeyModel, error) {
	keyHash = strings.TrimSpace(keyHash)
	if keyHash == "" {
		return nil, fmt.Errorf("not found")
	}
	var m model.SwapSvcApiKeyModel
	err := r.db.WithContext(ctx).Where("key_hash = ?", keyHash).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("not found")
	}
	return &m, err
}

func (r *swapApiKeyRepo) List(ctx context.Context, page, pageSize int32) ([]*model.SwapSvcApiKeyModel, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}

	query := r.db.WithContext(ctx).Model(&model.SwapSvcApiKeyModel{})

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := int((page - 1) * pageSize)
	var items []*model.SwapSvcApiKeyModel
	err := query.Order("created_at DESC").Offset(offset).Limit(int(pageSize)).Find(&items).Error
	return items, total, err
}

func (r *swapApiKeyRepo) ListByProjectName(ctx context.Context, projectName string) ([]*model.SwapSvcApiKeyModel, error) {
	projectName = strings.TrimSpace(projectName)
	if projectName == "" {
		return nil, fmt.Errorf("project_name is required")
	}
	var items []*model.SwapSvcApiKeyModel
	err := r.db.WithContext(ctx).
		Where("project_name = ?", projectName).
		Order("created_at DESC").
		Find(&items).Error
	return items, err
}

func (r *swapApiKeyRepo) UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error {
	if id <= 0 {
		return fmt.Errorf("not found")
	}
	fields["updated_at"] = time.Now().Local()
	res := r.db.WithContext(ctx).
		Model(&model.SwapSvcApiKeyModel{}).
		Where("id = ?", id).
		Updates(fields)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("not found")
	}
	return nil
}
