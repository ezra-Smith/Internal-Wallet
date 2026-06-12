package repository

import (
	"gorm.io/gorm"
	commonRepo "internalwallet/common/repository"
	"internalwallet/services/business/rpc/internal/model"
)

type WithdrawalAuditRepository interface {
	commonRepo.BaseRepository[model.WithdrawalAuditModel]
}

type withdrawalAuditRepo struct {
	commonRepo.BaseRepository[model.WithdrawalAuditModel]
}

func NewWithdrawalAuditRepository(db *gorm.DB) WithdrawalAuditRepository {
	return &withdrawalAuditRepo{
		BaseRepository: commonRepo.NewBaseRepository[model.WithdrawalAuditModel](db),
	}
}
