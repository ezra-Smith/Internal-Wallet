package repository

import (
	"context"

	commonRepo "internalwallet/common/repository"
	"internalwallet/services/admin/rpc/internal/model"

	"gorm.io/gorm"
)

type UserAdminNoteRepository interface {
	commonRepo.BaseRepository[model.UserAdminNoteModel]

	ListByUserID(ctx context.Context, userID int64, limit int) ([]*model.UserAdminNoteModel, error)
}

type userAdminNoteRepo struct {
	commonRepo.BaseRepository[model.UserAdminNoteModel]
}

func NewUserAdminNoteRepository(db *gorm.DB) UserAdminNoteRepository {
	return &userAdminNoteRepo{
		BaseRepository: commonRepo.NewBaseRepository[model.UserAdminNoteModel](db),
	}
}

func (r *userAdminNoteRepo) ListByUserID(ctx context.Context, userID int64, limit int) ([]*model.UserAdminNoteModel, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	var items []*model.UserAdminNoteModel
	err := r.GetDB().WithContext(ctx).
		Model(&model.UserAdminNoteModel{}).
		Where("user_id = ?", userID).
		Order("id DESC").
		Limit(limit).
		Find(&items).Error
	return items, err
}
