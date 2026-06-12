package repository

import (
	"gorm.io/gorm"
	commonRepo "internalwallet/common/repository"
	"internalwallet/services/business/rpc/internal/model"
)

type UserRoleRepository interface {
	commonRepo.BaseRepository[model.UserRoleModel]
}

type userRoleRepo struct {
	commonRepo.BaseRepository[model.UserRoleModel]
}

func NewUserRoleRepository(db *gorm.DB) UserRoleRepository {
	return &userRoleRepo{
		BaseRepository: commonRepo.NewBaseRepository[model.UserRoleModel](db),
	}
}
