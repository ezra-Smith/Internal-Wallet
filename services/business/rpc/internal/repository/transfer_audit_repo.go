package repository

import (
	commonRepo "internalwallet/common/repository"
	"internalwallet/services/business/rpc/internal/model"

	"gorm.io/gorm"
)

type TransferAuditRepository interface {
	commonRepo.BaseRepository[model.TransferAuditModel]
}

type transferAuditRepo struct {
	commonRepo.BaseRepository[model.TransferAuditModel]
}

func NewTransferAuditRepository(db *gorm.DB) TransferAuditRepository {
	return &transferAuditRepo{
		BaseRepository: commonRepo.NewBaseRepository[model.TransferAuditModel](db),
	}
}
