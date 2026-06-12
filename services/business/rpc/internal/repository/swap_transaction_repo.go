package repository

import (
	"gorm.io/gorm"
	commonRepo "internalwallet/common/repository"
	"internalwallet/services/business/rpc/internal/model"
)

type SwapTransactionRepository interface {
	commonRepo.BaseRepository[model.SwapTransactionModel]
}

type swapTransactionRepo struct {
	commonRepo.BaseRepository[model.SwapTransactionModel]
}

func NewSwapTransactionRepository(db *gorm.DB) SwapTransactionRepository {
	return &swapTransactionRepo{
		BaseRepository: commonRepo.NewBaseRepository[model.SwapTransactionModel](db),
	}
}
