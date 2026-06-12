package repository

import (
	"context"

	commonRepo "internalwallet/common/repository"
	"internalwallet/services/admin/rpc/internal/model"

	"gorm.io/gorm"
)

type AdminLoginLogRepository interface {
	commonRepo.BaseRepository[model.AdminLoginLogModel]
	CreateLog(ctx context.Context, m *model.AdminLoginLogModel) error
}

type adminLoginLogRepo struct {
	commonRepo.BaseRepository[model.AdminLoginLogModel]
}

func NewAdminLoginLogRepository(db *gorm.DB) AdminLoginLogRepository {
	return &adminLoginLogRepo{
		BaseRepository: commonRepo.NewBaseRepository[model.AdminLoginLogModel](db),
	}
}

func (r *adminLoginLogRepo) CreateLog(ctx context.Context, m *model.AdminLoginLogModel) error {
	return r.GetDB().WithContext(ctx).Create(m).Error
}
