package repository

import (
	"context"

	commonRepo "internalwallet/common/repository"
	"internalwallet/services/admin/rpc/internal/model"

	"gorm.io/gorm"
)

type AdminPasswordHistoryRepository interface {
	commonRepo.BaseRepository[model.AdminPasswordHistoryModel]

	Add(ctx context.Context, adminID int64, passwordHash string) error
	ListRecent(ctx context.Context, adminID int64, limit int) ([]*model.AdminPasswordHistoryModel, error)
}

type adminPasswordHistoryRepo struct {
	commonRepo.BaseRepository[model.AdminPasswordHistoryModel]
}

func NewAdminPasswordHistoryRepository(db *gorm.DB) AdminPasswordHistoryRepository {
	return &adminPasswordHistoryRepo{
		BaseRepository: commonRepo.NewBaseRepository[model.AdminPasswordHistoryModel](db),
	}
}

func (r *adminPasswordHistoryRepo) Add(ctx context.Context, adminID int64, passwordHash string) error {
	return r.GetDB().WithContext(ctx).Create(&model.AdminPasswordHistoryModel{
		AdminID:      adminID,
		PasswordHash: passwordHash,
	}).Error
}

func (r *adminPasswordHistoryRepo) ListRecent(ctx context.Context, adminID int64, limit int) ([]*model.AdminPasswordHistoryModel, error) {
	if limit <= 0 {
		limit = 5
	}
	var items []*model.AdminPasswordHistoryModel
	err := r.GetDB().WithContext(ctx).
		Where("admin_id = ?", adminID).
		Order("id DESC").
		Limit(limit).
		Find(&items).Error
	return items, err
}
