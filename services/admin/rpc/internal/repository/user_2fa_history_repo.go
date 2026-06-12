package repository

import (
	"context"
	"errors"
	"fmt"

	commonRepo "internalwallet/common/repository"
	"internalwallet/services/admin/rpc/internal/model"

	"gorm.io/gorm"
)

type User2FAHistoryRepository interface {
	commonRepo.BaseRepository[model.User2FAHistoryModel]

	Create(ctx context.Context, m *model.User2FAHistoryModel) error
	ListByUserID(ctx context.Context, userID int64, page, pageSize int32) ([]*model.User2FAHistoryModel, int64, error)
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

func (r *user2FAHistoryRepo) ListByUserID(ctx context.Context, userID int64, page, pageSize int32) ([]*model.User2FAHistoryModel, int64, error) {
	if userID <= 0 {
		return nil, 0, fmt.Errorf("invalid user_id")
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}

	query := r.GetDB().WithContext(ctx).Model(&model.User2FAHistoryModel{}).
		Where("user_id = ? AND deleted_at IS NULL", userID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return []*model.User2FAHistoryModel{}, 0, nil
		}
		return nil, 0, err
	}

	var items []*model.User2FAHistoryModel
	offset := int((page - 1) * pageSize)
	if err := query.Order("id DESC").Offset(offset).Limit(int(pageSize)).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
