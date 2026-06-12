package repository

import (
	commonRepo "internalwallet/common/repository"
	"internalwallet/services/business/rpc/internal/model"

	"gorm.io/gorm"
)

type UserLoginRecordRepository interface {
	commonRepo.BaseRepository[model.UserLoginRecordModel]
}

type userLoginRecordRepo struct {
	commonRepo.BaseRepository[model.UserLoginRecordModel]
}

func NewUserLoginRecordRepository(db *gorm.DB) UserLoginRecordRepository {
	return &userLoginRecordRepo{
		BaseRepository: commonRepo.NewBaseRepository[model.UserLoginRecordModel](db),
	}
}
