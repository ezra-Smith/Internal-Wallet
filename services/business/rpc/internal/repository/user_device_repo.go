package repository

import (
	"context"
	commonRepo "internalwallet/common/repository"
	"internalwallet/services/business/rpc/internal/model"

	"gorm.io/gorm"
)

type UserDeviceRepository interface {
	commonRepo.BaseRepository[model.UserDeviceModel]
	CreateDevice(ctx context.Context, dev *model.UserDeviceModel) error
	CountByUser(ctx context.Context, userID int64) (int64, error)
	ListDevicesByUser(ctx context.Context, userID int64, page int32, size int32) ([]model.UserDeviceModel, int64, error)
}

type userDeviceRepo struct {
	commonRepo.BaseRepository[model.UserDeviceModel]
}

func NewUserDeviceRepository(db *gorm.DB) UserDeviceRepository {
	return &userDeviceRepo{
		BaseRepository: commonRepo.NewBaseRepository[model.UserDeviceModel](db),
	}
}

func (r *userDeviceRepo) CreateDevice(ctx context.Context, dev *model.UserDeviceModel) error {
	return r.GetDB().WithContext(context.Background()).Create(dev).Error
}

func (r *userDeviceRepo) CountByUser(ctx context.Context, userID int64) (int64, error) {
	var count int64
	err := r.GetDB().WithContext(context.Background()).Model(&model.UserDeviceModel{}).Where("user_id = ?", userID).Count(&count).Error
	return count, err
}

func (r *userDeviceRepo) ListDevicesByUser(ctx context.Context, userID int64, page int32, size int32) ([]model.UserDeviceModel, int64, error) {
	q := r.GetDB().WithContext(context.Background()).Model(&model.UserDeviceModel{}).Where("user_id = ?", userID)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []model.UserDeviceModel
	err := q.Order("last_login_time DESC").Offset(int((page - 1) * size)).Limit(int(size)).Find(&rows).Error
	return rows, total, err
}
