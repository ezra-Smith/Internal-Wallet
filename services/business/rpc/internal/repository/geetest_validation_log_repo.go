package repository

import (
	"gorm.io/gorm"
	commonRepo "internalwallet/common/repository"
	"internalwallet/services/business/rpc/internal/model"
)

type GeetestValidationLogRepository interface {
	commonRepo.BaseRepository[model.GeetestValidationLogModel]
}

type geetestValidationLogRepo struct {
	commonRepo.BaseRepository[model.GeetestValidationLogModel]
}

func NewGeetestValidationLogRepository(db *gorm.DB) GeetestValidationLogRepository {
	return &geetestValidationLogRepo{
		BaseRepository: commonRepo.NewBaseRepository[model.GeetestValidationLogModel](db),
	}
}
