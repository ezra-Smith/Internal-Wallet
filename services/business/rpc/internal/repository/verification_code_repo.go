package repository

import (
	"gorm.io/gorm"
	commonRepo "internalwallet/common/repository"
	"internalwallet/services/business/rpc/internal/model"
)

type VerificationCodeRepository interface {
	commonRepo.BaseRepository[model.VerificationCodeModel]
}

type verificationCodeRepo struct {
	commonRepo.BaseRepository[model.VerificationCodeModel]
}

func NewVerificationCodeRepository(db *gorm.DB) VerificationCodeRepository {
	return &verificationCodeRepo{
		BaseRepository: commonRepo.NewBaseRepository[model.VerificationCodeModel](db),
	}
}
