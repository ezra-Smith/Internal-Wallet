package repository

import (
	"gorm.io/gorm"
	commonRepo "internalwallet/common/repository"
	"internalwallet/services/business/rpc/internal/model"
)

type PayrollRecordRepository interface {
	commonRepo.BaseRepository[model.PayrollRecordModel]
}

type payrollRecordRepo struct {
	commonRepo.BaseRepository[model.PayrollRecordModel]
}

func NewPayrollRecordRepository(db *gorm.DB) PayrollRecordRepository {
	return &payrollRecordRepo{
		BaseRepository: commonRepo.NewBaseRepository[model.PayrollRecordModel](db),
	}
}
