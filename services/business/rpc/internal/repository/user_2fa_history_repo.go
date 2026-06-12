package repository

import (
	"context"
	"fmt"

	commonRepo "internalwallet/common/repository"
	"internalwallet/services/business/rpc/internal/model"

	"gorm.io/gorm"
)

type User2FAHistoryRepository interface {
	commonRepo.BaseRepository[model.User2FAHistoryModel]

	Create(ctx context.Context, m *model.User2FAHistoryModel) error
}

type user2FAHistoryRepo struct {
	commonRepo.BaseRepository[model.User2FAHistoryModel]
}

func NewUser2FAHistoryRepository(db *gorm.DB) User2FAHistoryRepository {
	return &user2FAHistoryRepo{
		BaseRepository: commonRepo.NewBaseRepository[model.User2FAHistoryModel](db),
	}
}

func (r *user2FAHistoryRepo) Create(ctx context.Context, m *model.User2FAHistoryModel) error {
	if m == nil {
		return fmt.Errorf("nil model")
	}
	return r.GetDB().WithContext(ctx).Create(m).Error
}
