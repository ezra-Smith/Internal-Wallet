package repository

import (
	"gorm.io/gorm"
	commonRepo "internalwallet/common/repository"
	"internalwallet/services/business/rpc/internal/model"
)

type TrustedDeviceRepository interface {
	commonRepo.BaseRepository[model.TrustedDeviceModel]
}

type trustedDeviceRepo struct {
	commonRepo.BaseRepository[model.TrustedDeviceModel]
}

func NewTrustedDeviceRepository(db *gorm.DB) TrustedDeviceRepository {
	return &trustedDeviceRepo{
		BaseRepository: commonRepo.NewBaseRepository[model.TrustedDeviceModel](db),
	}
}
