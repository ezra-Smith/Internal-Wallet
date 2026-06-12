package repository

import (
	"gorm.io/gorm"
	commonRepo "internalwallet/common/repository"
	"internalwallet/services/business/rpc/internal/model"
)

type LoginLogRepository interface {
	commonRepo.BaseRepository[model.LoginLogModel]
}

type loginLogRepo struct {
	commonRepo.BaseRepository[model.LoginLogModel]
}

func NewLoginLogRepository(db *gorm.DB) LoginLogRepository {
	return &loginLogRepo{
		BaseRepository: commonRepo.NewBaseRepository[model.LoginLogModel](db),
	}
}
